package main

import (
	"context"
	"errors"
	"image"
	"image/draw"
	"math"
	"sync"
)

type filterTap struct {
	Index  int
	Weight float32
}

func resizeTaps(src, dst int) [][]filterTap {
	out := make([][]filterTap, dst)
	ratio := float64(src) / float64(dst)
	scale := math.Max(1, ratio)
	for x := range out {
		center := (float64(x)+.5)*ratio - .5
		sum := 0.0
		for i := int(math.Ceil(center - 3*scale)); i <= int(math.Floor(center+3*scale)); i++ {
			d := (center - float64(i)) / scale
			w := 1.0
			if math.Abs(d) > 1e-8 {
				w = 3 * math.Sin(math.Pi*d) * math.Sin(math.Pi*d/3) / (math.Pi * math.Pi * d * d)
			}
			if math.Abs(d) >= 3 {
				w = 0
			}
			if w == 0 {
				continue
			}
			out[x] = append(out[x], filterTap{maxInt(0, minInt(src-1, i)), float32(w)})
			sum += w
		}
		for k := range out[x] {
			out[x][k].Weight /= float32(sum)
		}
	}
	return out
}

// Separable Lanczos-3 with a small row cache per worker. No full float image.
// Work is divided into contiguous strips; cancellation is checked every row.
func resizeFinish(ctx context.Context, src *image.NRGBA, w, h, workers int) (*image.NRGBA, error) {
	if w < 1 || h < 1 || w > 7680 || h > 7680 || int64(w)*int64(h) > 60_000_000 {
		return nil, errors.New("output exceeds the 7680-pixel / 60-megapixel limit")
	}
	if src.Bounds().Dx() < 1 || src.Bounds().Dy() < 1 {
		return nil, errors.New("empty source")
	}
	dst := image.NewNRGBA(image.Rect(0, 0, w, h))
	xt, yt := resizeTaps(src.Bounds().Dx(), w), resizeTaps(src.Bounds().Dy(), h)
	workers = maxInt(1, minInt(8, minInt(workers, h)))
	var wg sync.WaitGroup
	for worker := 0; worker < workers; worker++ {
		start, end := h*worker/workers, h*(worker+1)/workers
		wg.Add(1)
		go func() {
			defer wg.Done()
			cache := map[int][]float32{}
			for y := start; y < end; y++ {
				if ctx.Err() != nil {
					return
				}
				needed := map[int]bool{}
				for _, v := range yt[y] {
					needed[v.Index] = true
					if _, ok := cache[v.Index]; ok {
						continue
					}
					row := make([]float32, w*4)
					for x, taps := range xt {
						for _, t := range taps {
							off := src.PixOffset(t.Index+src.Rect.Min.X, v.Index+src.Rect.Min.Y)
							a := float32(src.Pix[off+3]) / 255
							for c := 0; c < 3; c++ {
								row[x*4+c] += float32(src.Pix[off+c]) * a * t.Weight
							}
							row[x*4+3] += float32(src.Pix[off+3]) * t.Weight
						}
					}
					cache[v.Index] = row
				}
				for i := range cache {
					if !needed[i] {
						delete(cache, i)
					}
				}
				for x := 0; x < w; x++ {
					var c [4]float32
					for _, t := range yt[y] {
						r := cache[t.Index]
						for k := 0; k < 4; k++ {
							c[k] += r[x*4+k] * t.Weight
						}
					}
					off := dst.PixOffset(x, y)
					alpha := math.Max(0, math.Min(255, float64(c[3])))
					dst.Pix[off+3] = byte(math.Round(alpha))
					if alpha > 0 {
						for k := 0; k < 3; k++ {
							dst.Pix[off+k] = byte(math.Round(math.Max(0, math.Min(255, float64(c[k])*255/alpha))))
						}
					}
				}
			}
		}()
	}
	wg.Wait()
	return dst, ctx.Err()
}
func finishNRGBA(src image.Image) *image.NRGBA {
	b := src.Bounds()
	out := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(out, out.Bounds(), src, b.Min, draw.Src)
	return out
}
func finishDimensions(w, h, long int) (int, int, error) {
	if long < 64 || long > 7680 {
		return 0, 0, errors.New("choose a long edge between 64 and 7680 pixels")
	}
	if w < 1 || h < 1 {
		return 0, 0, errors.New("invalid source dimensions")
	}
	ratio := float64(long) / float64(maxInt(w, h))
	ow, oh := maxInt(1, int(math.Round(float64(w)*ratio))), maxInt(1, int(math.Round(float64(h)*ratio)))
	if int64(ow)*int64(oh) > 60_000_000 {
		return 0, 0, errors.New("output exceeds 60 megapixels")
	}
	return ow, oh, nil
}

// A bounded, edge-aware unsharp pass. Keep three original rows; preserve alpha.
func sharpenFinish(ctx context.Context, im *image.NRGBA, amount float64) error {
	if amount <= 0 {
		return nil
	}
	w, h := im.Bounds().Dx(), im.Bounds().Dy()
	stride := w * 4
	previous := append([]byte{}, im.Pix[:stride]...)
	current := append([]byte{}, previous...)
	for y := 0; y < h; y++ {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		next := append([]byte{}, im.Pix[minInt(y+1, h-1)*im.Stride:minInt(y+1, h-1)*im.Stride+stride]...)
		for x := 0; x < w; x++ {
			for c := 0; c < 3; c++ {
				i := x*4 + c
				v := float64(current[i])
				blur := (float64(current[maxInt(0, x-1)*4+c]) + float64(current[minInt(w-1, x+1)*4+c]) + float64(previous[i]) + float64(next[i])) / 4
				delta := v - blur
				if math.Abs(delta) > 2 {
					delta = math.Max(-16, math.Min(16, delta*amount))
					im.Pix[y*im.Stride+i] = byte(math.Round(math.Max(0, math.Min(255, v+delta))))
				}
			}
		}
		previous, current = current, next
	}
	return nil
}
