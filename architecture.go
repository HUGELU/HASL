package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"image/png"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

type PlanRoom struct {
	Name       string  `json:"name"`
	X          float64 `json:"x"`
	Y          float64 `json:"y"`
	W          float64 `json:"w"`
	H          float64 `json:"h"`
	Door       string  `json:"door"`
	DoorOffset float64 `json:"door_offset"`
	DoorWidth  float64 `json:"door_width"`
}
type ArchitectureView struct {
	Asset   string `json:"asset"`
	Label   string `json:"label"`
	Room    string `json:"room,omitempty"`
	Seconds int    `json:"seconds"`
}
type ArchitectureProject struct {
	Schema string             `json:"schema"`
	Name   string             `json:"name"`
	Rooms  []PlanRoom         `json:"rooms"`
	Views  []ArchitectureView `json:"views"`
}

func validateArchitecture(p ArchitectureProject) error {
	if len(p.Name) > 100 || strings.TrimSpace(p.Name) == "" || len(p.Rooms) > 30 || len(p.Views) > 12 {
		return errors.New("use a project name, at most 30 rooms and 12 views")
	}
	for i, r := range p.Rooms {
		for _, v := range []float64{r.X, r.Y, r.W, r.H, r.DoorOffset, r.DoorWidth} {
			if math.IsNaN(v) || math.IsInf(v, 0) {
				return errors.New("dimensions must be finite")
			}
		}
		if len(r.Name) > 80 || r.Name == "" || r.X < 0 || r.Y < 0 || r.W < .5 || r.H < .5 || r.X+r.W > 200 || r.Y+r.H > 200 {
			return fmt.Errorf("room %d needs valid metre dimensions within 200 m", i+1)
		}
		if r.Door != "" {
			length := r.W
			if r.Door == "left" || r.Door == "right" {
				length = r.H
			} else if r.Door != "top" && r.Door != "bottom" {
				return errors.New("door wall must be top, bottom, left or right")
			}
			if r.DoorWidth < .5 || r.DoorWidth > 2.4 || r.DoorOffset < 0 || r.DoorOffset+r.DoorWidth > length {
				return errors.New("door opening must fit its room wall")
			}
		}
		for j := 0; j < i; j++ {
			b := p.Rooms[j]
			if r.Name == b.Name { return errors.New("use distinct room names so reference views can identify their room") }
			if math.Min(r.X+r.W, b.X+b.W)-math.Max(r.X, b.X) > 1e-6 && math.Min(r.Y+r.H, b.Y+b.H)-math.Max(r.Y, b.Y) > 1e-6 {
				return fmt.Errorf("rooms %s and %s overlap", r.Name, b.Name)
			}
		}
	}
	for _, v := range p.Views {
		if len(v.Asset) != 64 || len(v.Label) > 100 || v.Seconds < 1 || v.Seconds > 10 {
			return errors.New("each view needs a stored image and 1–10 second duration")
		}
		if v.Room != "" {
			found := false
			for _, r := range p.Rooms { if r.Name == v.Room { found = true } }
			if !found { return errors.New("a view refers to a room missing from this project") }
		}
	}
	return nil
}
func planSVG(p ArchitectureProject) (string, float64, error) {
	if err := validateArchitecture(p); err != nil {
		return "", 0, err
	}
	if len(p.Rooms) == 0 {
		return "", 0, errors.New("add at least one dimensioned room")
	}
	w, h, area := 0.0, 0.0, 0.0
	for _, r := range p.Rooms {
		w = math.Max(w, r.X+r.W)
		h = math.Max(h, r.Y+r.H)
		area += r.W * r.H
	}
	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="-1 -1.8 %.3f %.3f" width="1200" role="img"><title>%s</title><rect x="-1" y="-1.8" width="100%%" height="100%%" fill="white"/><g fill="#111" font-family="Arial,sans-serif"><text x="0" y="-1" font-size=".38">%s</text><text x="0" y="-.55" font-size=".2">User-entered dimensions · metres · schematic plan</text>`, w+2, h+3.1, html.EscapeString(p.Name), html.EscapeString(p.Name))
	for _, r := range p.Rooms {
		fmt.Fprintf(&b, `<rect x="%.3f" y="%.3f" width="%.3f" height="%.3f" fill="#f7f7f5" stroke="#111" stroke-width=".05"/><text x="%.3f" y="%.3f" text-anchor="middle" font-size=".23">%s</text><text x="%.3f" y="%.3f" text-anchor="middle" font-size=".18">%.2f × %.2f m · %.2f m²</text>`, r.X, r.Y, r.W, r.H, r.X+r.W/2, r.Y+r.H/2-.12, html.EscapeString(r.Name), r.X+r.W/2, r.Y+r.H/2+.16, r.W, r.H, r.W*r.H)
		if r.Door != "" {
			x, y, angle := r.X+r.DoorOffset, r.Y, 0
			if r.Door == "bottom" {
				y = r.Y + r.H
				angle = 180
				x = r.X + r.DoorOffset + r.DoorWidth
			}
			if r.Door == "left" {
				x = r.X
				y = r.Y + r.DoorOffset + r.DoorWidth
				angle = -90
			}
			if r.Door == "right" {
				x = r.X + r.W
				y = r.Y + r.DoorOffset
				angle = 90
			}
			fmt.Fprintf(&b, `<g transform="translate(%.3f %.3f) rotate(%d)"><path d="M0 0 H%.3f" stroke="white" stroke-width=".07"/><path d="M0 0 V%.3f M%.3f 0 A%.3f %.3f 0 0 1 0 %.3f" fill="none" stroke="#666" stroke-width=".02"/></g>`, x, y, angle, r.DoorWidth, r.DoorWidth, r.DoorWidth, r.DoorWidth, r.DoorWidth, r.DoorWidth)
		}
	}
	fmt.Fprintf(&b, `<text x="0" y="%.3f" font-size=".19">Room area sum: %.2f m² · overall bounds: %.2f × %.2f m</text><text x="0" y="%.3f" font-size=".17">Verify dimensions on site. Wall thickness and statutory floor-area rules are not calculated.</text></g></svg>`, h+.5, area, w, h, h+.85)
	return b.String(), area, nil
}
func (e *Engine) architectureRecipe(p ArchitectureProject) (AssetRecord, error) {
	p.Schema = "origin0.architecture.v1"
	b, _ := json.MarshalIndent(p, "", "  ")
	a, err := e.storeObject(bytes.NewReader(b), "ORIGIN0_ARCHITECTURE.json", "application/json", "user-dimensioned architecture project")
	if err == nil {
		err = atomicWrite(filepath.Join(e.dataDir, "architecture.json"), b)
	}
	return a, err
}
func (e *Engine) walkthroughHTML(ctx context.Context, p ArchitectureProject) (string, error) {
	if err := validateArchitecture(p); err != nil {
		return "", err
	}
	if len(p.Views) < 2 {
		return "", errors.New("add at least two reference views")
	}
	type frame struct {
		Image   string `json:"image"`
		Label   string `json:"label"`
		Seconds int    `json:"seconds"`
	}
	frames := []frame{}
	total := 0
	for _, v := range p.Views {
		src, err := e.finishing.source(v.Asset)
		if err != nil {
			return "", err
		}
		small := src
		if maxInt(src.Rect.Dx(), src.Rect.Dy()) > 1920 {
			w, h, dimErr := finishDimensions(src.Rect.Dx(), src.Rect.Dy(), 1920)
			if dimErr != nil {
				return "", dimErr
			}
			small, err = resizeFinish(ctx, src, w, h, 2)
			if err != nil {
				return "", err
			}
		}
		var buf bytes.Buffer
		enc := png.Encoder{CompressionLevel: png.BestSpeed}
		if err = enc.Encode(&buf, small); err != nil {
			return "", err
		}
		total += buf.Len()
		if total > 48<<20 {
			return "", errors.New("walkthrough exceeds 48 MiB of embedded images")
		}
		label := v.Label
		if v.Room != "" { label = v.Room + " · " + label }
		frames = append(frames, frame{"data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes()), label, v.Seconds})
	}
	data, _ := json.Marshal(frames)
	parts := strings.SplitN(walkthroughTemplate, "__FRAMES__", 2)
	page := strings.ReplaceAll(parts[0], "__TITLE__", html.EscapeString(p.Name)) + string(data) + strings.ReplaceAll(parts[1], "__PLAYER__", walkthroughScript)
	return page, nil
}

const walkthroughTemplate = `<!doctype html><html lang="en"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>__TITLE__</title><style>body{margin:0;background:#151715;color:#eee;font:16px system-ui}main{max-width:1280px;margin:auto;padding:20px}canvas{width:100%;height:auto;background:#111}button{padding:12px;margin:4px;border:1px solid #888;border-radius:8px;background:#262c26;color:white}p{line-height:1.5}progress{width:100%}</style><main><h1>__TITLE__</h1><p>Photo walkthrough · an ordered sequence of supplied views, with gentle motion. This does not reconstruct a navigable 3D scene.</p><canvas id="canvas" width="1920" height="1080"></canvas><progress id="progress" max="1" value="0"></progress><div><button id="play">Play</button><button id="record">Export WebM video</button><button id="stop">Stop</button></div><p id="status" role="status">Ready. Video export plays in real time; keep this tab visible.</p></main><script type="application/json" id="frames">__FRAMES__</script><script>__PLAYER__</script></html>`

const walkthroughScript = `const frames=JSON.parse(document.getElementById('frames').textContent),canvas=document.getElementById('canvas'),ctx=canvas.getContext('2d'),status=document.getElementById('status');let raf=0,rec=null,stream=null,running=false;const total=frames.reduce((a,f)=>a+f.seconds,0),images=frames.map(f=>{const i=new Image();i.src=f.image;return i});
function draw(t){let start=0,k=0;while(k<frames.length-1&&t>start+frames[k].seconds){start+=frames[k].seconds;k++}const f=frames[k],im=images[k],u=Math.min(1,(t-start)/f.seconds),scale=Math.min(canvas.width/im.width,canvas.height/im.height)*(1+.025*u);ctx.fillStyle='#111';ctx.fillRect(0,0,canvas.width,canvas.height);ctx.drawImage(im,(canvas.width-im.width*scale)/2,(canvas.height-im.height*scale)/2,im.width*scale,im.height*scale);ctx.fillStyle='rgba(0,0,0,.65)';ctx.fillRect(0,990,1920,90);ctx.fillStyle='white';ctx.font='30px sans-serif';ctx.fillText(f.label,40,1045,1840);document.getElementById('progress').value=Math.min(1,t/total)}
async function start(record){if(running)return;await Promise.all(images.map(i=>i.decode()));if(record&&(!window.MediaRecorder||!MediaRecorder.isTypeSupported('video/webm;codecs=vp9'))){status.textContent='This browser does not support VP9 WebM recording. Playback remains available.';return}let chunks=[];if(record){stream=canvas.captureStream(24);rec=new MediaRecorder(stream,{mimeType:'video/webm;codecs=vp9',videoBitsPerSecond:8000000});rec.ondataavailable=e=>{if(e.data.size)chunks.push(e.data)};rec.onstop=()=>{const u=URL.createObjectURL(new Blob(chunks,{type:'video/webm'})),a=document.createElement('a');a.href=u;a.download='ORIGIN0_WALKTHROUGH.webm';a.click();setTimeout(()=>URL.revokeObjectURL(u),30000);stream.getTracks().forEach(t=>t.stop());status.textContent='Video export saved. Review playback before sharing.'};rec.start(1000)}running=true;const epoch=performance.now();status.textContent=record?'Recording locally in real time…':'Playing…';function tick(now){const t=(now-epoch)/1000;draw(Math.min(t,total));if(t<total&&running){raf=requestAnimationFrame(tick)}else stop()}raf=requestAnimationFrame(tick)}
function stop(){running=false;cancelAnimationFrame(raf);if(rec?.state==='recording')rec.stop();else status.textContent='Stopped.'}document.getElementById('play').onclick=()=>start(false).catch(e=>status.textContent=e.message);document.getElementById('record').onclick=()=>start(true).catch(e=>status.textContent=e.message);document.getElementById('stop').onclick=stop;document.addEventListener('visibilitychange',()=>{if(document.hidden&&running){stop();status.textContent='Stopped because the tab became hidden. Export may be partial.'}});Promise.all(images.map(i=>i.decode())).then(()=>draw(0));
`

func walkthroughScriptHash() string { sum := sha256.Sum256([]byte(walkthroughScript)); return base64.StdEncoding.EncodeToString(sum[:]) }

func (e *Engine) architectureRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/architecture/save", func(w http.ResponseWriter, r *http.Request) {
		var p ArchitectureProject
		if err := decode(r, &p); err != nil { apiError(w, err); return }
		if err := validateArchitecture(p); err != nil { apiError(w, err); return }
		for _, v := range p.Views { if _, err := e.objectPath(v.Asset); err != nil { apiError(w, err); return } }
		a, err := e.architectureRecipe(p)
		if err != nil { apiError(w, err); return }
		jsonReply(w, a)
	})
	mux.HandleFunc("/api/architecture/project", func(w http.ResponseWriter, r *http.Request) {
		b, err := os.ReadFile(filepath.Join(e.dataDir, "architecture.json"))
		if err != nil {
			jsonReply(w, ArchitectureProject{Name: "My property"})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(b)
	})
	mux.HandleFunc("/api/architecture/plan", func(w http.ResponseWriter, r *http.Request) {
		var p ArchitectureProject
		if err := decode(r, &p); err != nil {
			apiError(w, err)
			return
		}
		svg, area, err := planSVG(p)
		if err != nil {
			apiError(w, err)
			return
		}
		a, err := e.storeObject(strings.NewReader(svg), "ORIGIN0_FLOOR_PLAN.svg", "image/svg+xml", "schematic from user dimensions")
		if err != nil {
			apiError(w, err)
			return
		}
		recipe, err := e.architectureRecipe(p)
		if err != nil {
			apiError(w, err)
			return
		}
		jsonReply(w, map[string]any{"svg": a, "recipe": recipe, "room_area": area})
	})
	mux.HandleFunc("/api/architecture/walkthrough", func(w http.ResponseWriter, r *http.Request) {
		var p ArchitectureProject
		if err := decode(r, &p); err != nil {
			apiError(w, err)
			return
		}
		page, err := e.walkthroughHTML(r.Context(), p)
		if err != nil {
			apiError(w, err)
			return
		}
		a, err := e.storeObject(strings.NewReader(page), "ORIGIN0_PHOTO_WALKTHROUGH.html", "text/html", "self-contained photo walkthrough")
		if err != nil {
			apiError(w, err)
			return
		}
		recipe, err := e.architectureRecipe(p)
		if err != nil {
			apiError(w, err)
			return
		}
		jsonReply(w, map[string]any{"html": a, "recipe": recipe})
	})
}
