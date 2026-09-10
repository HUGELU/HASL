// ORIGIN-0 MIT. Inference uses NCNN and the unmodified Real-ESRGAN x4plus model.
// A deliberately small raw-RGB contract keeps image parsing in the Go host.
#include <net.h>
#include <gpu.h>
#include <algorithm>
#include <fstream>
#include <iostream>
#include <string>
#include <vector>
#include <cmath>

int main(int argc, char** argv) {
    try {
        if (argc != 10) { std::cerr << "input.rgb output.rgb width height model.param model.bin tile threads cpu|auto\n"; return 2; }
        int w=std::stoi(argv[3]), h=std::stoi(argv[4]), tile=std::stoi(argv[7]), threads=std::stoi(argv[8]);
        std::string mode=argv[9];
        if (w<1 || h<1 || w>2048 || h>2048 || (long long)w*h>4194304 || tile<32 || tile>256 || threads<1 || threads>64 || (mode!="cpu" && mode!="auto")) return 2;
        std::vector<unsigned char> input((size_t)w*h*3), output((size_t)w*h*3*16);
        std::ifstream f(argv[1],std::ios::binary); f.read((char*)input.data(),input.size());
        if ((size_t)f.gcount()!=input.size() || f.peek()!=EOF) { std::cerr << "invalid RGB input\n"; return 3; }
        ncnn::create_gpu_instance();
        bool gpu=mode=="auto" && ncnn::get_gpu_count()>0;
        {
            ncnn::Net net;
            net.opt.num_threads=threads;
            net.opt.use_vulkan_compute=gpu;
            net.opt.use_fp16_arithmetic=false;
            net.opt.use_fp16_storage=gpu;
            if (gpu) net.set_vulkan_device(ncnn::get_default_gpu_index());
            if (net.load_param(argv[5]) || net.load_model(argv[6])) { std::cerr << "model load failed\n"; return 4; }
            std::cout << "backend=" << (gpu?"vulkan":"cpu") << "\n" << std::flush;
            const int halo=32, total=((w+tile-1)/tile)*((h+tile-1)/tile); int done=0;
            for (int y=0;y<h;y+=tile) for (int x=0;x<w;x+=tile) {
                int x0=std::max(0,x-halo), y0=std::max(0,y-halo), x1=std::min(w,x+tile+halo), y1=std::min(h,y+tile+halo);
                int tw=x1-x0, th=y1-y0;
                std::vector<unsigned char> region((size_t)tw*th*3);
                for(int yy=0;yy<th;yy++) std::copy_n(input.data()+((size_t)(y0+yy)*w+x0)*3,tw*3,region.data()+(size_t)yy*tw*3);
                ncnn::Mat in=ncnn::Mat::from_pixels(region.data(),ncnn::Mat::PIXEL_RGB,tw,th);
                const float norm[3]={1.f/255,1.f/255,1.f/255}; in.substract_mean_normalize(nullptr,norm);
                auto ex=net.create_extractor(); ncnn::Mat out;
                if(ex.input("data",in) || ex.extract("output",out) || out.w!=tw*4 || out.h!=th*4 || out.c!=3) { std::cerr<<"inference shape failure\n";return 5; }
                int cw=std::min(tile,w-x)*4, ch=std::min(tile,h-y)*4, ox=(x-x0)*4, oy=(y-y0)*4;
                for(int yy=0;yy<ch;yy++) for(int c=0;c<3;c++) {
                    const float* row=out.channel(c).row(yy+oy);
                    for(int xx=0;xx<cw;xx++) {
                        float value=row[xx+ox]; if(!std::isfinite(value)){std::cerr<<"nonfinite output\n";return 6;}
                        output[((size_t)(y*4+yy)*(w*4)+x*4+xx)*3+c]=(unsigned char)std::lround(std::clamp(value,0.f,1.f)*255);
                    }
                }
                std::cout<<"tile="<<++done<<"/"<<total<<"\n"<<std::flush;
            }
        }
        ncnn::destroy_gpu_instance();
        std::ofstream result(argv[2],std::ios::binary);result.write((char*)output.data(),output.size());
        if(!result){std::cerr<<"output write failed\n";return 7;}
        return 0;
    }catch(const std::exception& e){std::cerr<<e.what()<<"\n";return 8;}
}
