package main

import (
	"flag"
	"fmt"
	img "image"
	"image/color"
	"image/color/palette"
	"image/png"
	"log"
	"math"
	"math/rand"
	"net/http"
	"os"
	"sort"
	"strconv"
	"sync"

	"github.com/nfnt/resize"
)

const RADIUS_MIN = 0.0005
const RADIUS_MAX = 1.0

var IT, xres, yres, aa int
var xpos, ypos, radius float64
var out_filename, palette_string string
var invert, server_mode, randomize bool
var focusstring string
var port string

func init() {
	flag.IntVar(&IT, "IT", 512, "maximum number of iterations")
	flag.IntVar(&xres, "xres", 500, "x resolution")
	flag.IntVar(&yres, "yres", 500, "y resolution")
	flag.IntVar(&aa, "aa", 1, "anti alias, e.g. set aa=4 for 4xAA")
	flag.Float64Var(&xpos, "x", -.75, "real coordinate")
	flag.Float64Var(&ypos, "y", 0.0, "imaginary coordinate")
	flag.Float64Var(&radius, "r", 3.0, "radius")
	flag.StringVar(&out_filename, "out", "out.png", "output file")
	flag.StringVar(&palette_string, "palette", "plan9", "One of: plan9|websafe|gameboy|retro|alternate")
	flag.StringVar(&focusstring, "focus", "", "sequence of focus command. Select quadrant (numbered 1-4). e.g.: 1423. Read code to understand")
	flag.BoolVar(&invert, "invert", false, "Inverts colouring")
	flag.BoolVar(&server_mode, "server", false, "Enable web server mode (i.e., deliver resulting image over the web)")
	flag.StringVar(&port, "port", "8080", "listening port when server_mode == true")
	flag.BoolVar(&randomize, "random", false, "Use a random value for the radius parameter")
	flag.Parse()

	if randomize {
		randomize_params()
	}

	Gray = make([]color.Color, 255*3)
	for i := 0; i < 255*3; i++ {
		Gray[i] = color.RGBA{uint8(i / 3), uint8((i + 1) / 3), uint8((i + 2) / 3), 255}
	}

	Alternate = make([]color.Color, 20)
	for i := 0; i < len(Alternate); i++ {
		switch i % 6 {
		case 0:
			Alternate[i] = color.RGBA{0x18, 0x4d, 0x68, 255}
		case 1:
			Alternate[i] = color.RGBA{0x31, 0x80, 0x9f, 255}
		case 2:
			Alternate[i] = color.RGBA{0xfb, 0x9c, 0x6c, 255}
		case 3:
			Alternate[i] = color.RGBA{0xd5, 0x51, 0x21, 255}
		case 4:
			Alternate[i] = color.RGBA{0xcf, 0xe9, 0x90, 255}
		case 5:
			Alternate[i] = color.RGBA{0xea, 0xfb, 0xc5, 255}
		}
	}

	BlackWhite = make([]color.Color, 0)
	for i := 0; i < 20; i++ {
		if i%2 == 0 {
			BlackWhite = append(BlackWhite, color.RGBA{0, 0, 0, 255})
		} else {
			BlackWhite = append(BlackWhite, color.RGBA{255, 255, 255, 255})
		}
	}
}

func it(ca, cb float64) (int, float64) {
	var a, b float64 = 0, 0
	for i := 0; i < IT; i++ {
		as, bs := a*a, b*b
		if as+bs > 4 {
			return i, as + bs
		}
		//if as + bs < .00001 {
		//	return .00001
		//}
		a, b = as-bs+ca, 2*a*b+cb
	}
	return IT, a*a + b*b
}

var Gameboy = []color.Color{
	color.RGBA{14, 55, 15, 255},
	color.RGBA{47, 97, 48, 255},
	color.RGBA{138, 171, 25, 255},
	color.RGBA{154, 187, 27, 255},
}

var Retro = []color.Color{
	color.RGBA{0x00, 0x04, 0x0f, 0xff},
	color.RGBA{0x03, 0x26, 0x28, 0xff},
	color.RGBA{0x07, 0x3e, 0x1e, 0xff},
	color.RGBA{0x18, 0x55, 0x08, 0xff},
	color.RGBA{0x5f, 0x6e, 0x0f, 0xff},
	color.RGBA{0x84, 0x50, 0x19, 0xff},
	color.RGBA{0x9b, 0x30, 0x22, 0xff},
	color.RGBA{0xb4, 0x92, 0x2f, 0xff},
	color.RGBA{0x94, 0xca, 0x3d, 0xff},
	color.RGBA{0x4f, 0xd5, 0x51, 0xff},
	color.RGBA{0x66, 0xff, 0xb3, 0xff},
	color.RGBA{0x82, 0xc9, 0xe5, 0xff},
	color.RGBA{0x9d, 0xa3, 0xeb, 0xff},
	color.RGBA{0xd7, 0xb5, 0xf3, 0xff},
	color.RGBA{0xfd, 0xd6, 0xf6, 0xff},
	color.RGBA{0xff, 0xf0, 0xf2, 0xff},
}

var Gray, Alternate, BlackWhite []color.Color

func create_image() img.Image {

	width, height := xres*aa, yres*aa
	ratio := float64(height) / float64(width)
	//xpos, ypos, zoom_width := -.16070135, 1.0375665, 1.0e-7
	//xpos, ypos, zoom_width := -.7453, .1127, 6.5e-4
	//xpos, ypos, zoom_width := 0.45272105023, 0.396494224267,  .3E-9
	//xpos, ypos, zoom_width := -.160568374422, 1.037894847008, .000001
	//xpos, ypos, zoom_width := .232223859135, .559654166164, .00000000004
	y_radius := float64(radius * ratio)

	temp_radius, temp_y_radius := radius/4.0, y_radius/4.0
	for _, command := range focusstring {
		switch string(command) {
		case "1":
			xpos -= temp_radius
			ypos += temp_radius
		case "2":
			xpos += temp_radius
			ypos += temp_radius
		case "3":
			xpos -= temp_radius
			ypos -= temp_radius
		case "4":
			xpos += temp_radius
			ypos -= temp_radius
		case "w":
			ypos += temp_radius
		case "s":
			ypos -= temp_radius
		case "a":
			xpos -= temp_radius
		case "d":
			xpos += temp_radius
		case "r":
			temp_radius, temp_y_radius = radius/4, y_radius/4
		case "z":
			radius /= 2
			y_radius /= 2
			temp_radius, temp_y_radius = radius/4, y_radius/4
		default:
			return nil
		}
		temp_radius /= 2
		temp_y_radius /= 2
	}

	xmin, xmax := xpos-radius/2.0, xpos+radius/2.0
	ymin, ymax := ypos-y_radius/2.0, ypos+y_radius/2.0

	single_values := make([]float64, width*height)

	fmt.Print("Mandelling...")

	var wg sync.WaitGroup

	for y := 0; y < height; y++ {
		wg.Add(1)
		go func(y int) {
			defer wg.Done()
			for x := 0; x < width; x++ {
				a := (float64(x)/float64(width))*(xmax-xmin) + xmin
				b := (float64(y)/float64(height))*(ymin-ymax) + ymax
				stop_it, norm := it(a, b)
				smooth_val := float64(IT-stop_it) + math.Log(norm)
				i := y*width + x
				if invert {
					single_values[i] = smooth_val
				} else {
					single_values[i] = -smooth_val
				}
			}
		}(y)
	}
	wg.Wait()
	fmt.Println("Done")
	fmt.Print("Sorting...")
	sorted_values := make([]float64, len(single_values))
	for i := range sorted_values {
		sorted_values[i] = single_values[i]
	}
	sort.Float64s(sorted_values)

	fmt.Println("Done")

	cont := make([]color.Color, 10000)
	for i, _ := range cont {
		//val := float64(i) / float64(len(cont))
		val := i * 256 / len(cont)
		cont[i] = color.RGBA{uint8(val), 0, uint8(255 - val), uint8(255)}
	}

	var pal []color.Color
	palette_map := make(map[string][]color.Color)
	palette_map["plan9"] = palette.Plan9
	palette_map["websafe"] = palette.WebSafe
	palette_map["gameboy"] = Gameboy
	palette_map["retro"] = Retro
	palette_map["gray"] = Gray
	palette_map["cont"] = cont
	palette_map["alternate"] = Alternate
	palette_map["blackwhite"] = BlackWhite

	pal = palette_map[palette_string]

	split_values := make([]float64, len(pal)-1)

	factor := .98
	start := .9
	for i := range split_values {
		index := (i + 1) * len(sorted_values) / len(pal)
		//index := int(float64(len(sorted_values)-1) * (1.0 - start))
		split_values[i] = sorted_values[index]
		start *= factor
	}
	sort.Float64s(split_values)

	image := img.NewRGBA(img.Rectangle{img.Point{0, 0}, img.Point{width, height}})

	fmt.Print("Filling...")

	i := 0
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			color_index := sort.Search(len(split_values), func(j int) bool { return single_values[i] < split_values[j] })
			image.Set(x, y, pal[color_index])
			i++
		}
	}
	fmt.Println("Done")

	fmt.Println("Resizing...")
	image_resized := resize.Resize(uint(xres), uint(yres), image, resize.Lanczos3)
	fmt.Println("Done")

	return image_resized

}

func handler(w http.ResponseWriter, r *http.Request) {

	// Overide (if present) global vars from query parameters
	params := r.URL.Query()
	for k, v := range params {
		switch k {
		// TODO: add error handling code
		case "IT":
			IT, _ = strconv.Atoi(v[0])
		case "xres":
			xres, _ = strconv.Atoi(v[0])
		case "yres":
			yres, _ = strconv.Atoi(v[0])
		case "aa":
			aa, _ = strconv.Atoi(v[0])
		// // case "xpos":
		// // 	xpos, _ = strconv.ParseFloat(v[0], 64)
		// // case "ypos":
		// // 	ypos, _ = strconv.ParseFloat(v[0], 64)
		// case "radius":
		// 	radius, _ = strconv.ParseFloat(v[0], 64)
		case "palette":
			palette_string = v[0]
		case "focus":
			focusstring = v[0]
		case "invert":
			invert, _ = strconv.ParseBool(v[0])
		// case "random":
		// 	randomize, _ = strconv.ParseBool(v[0])
		// 	if randomize {
		// 		randomize_params()
		// 	}
		}
	}

	randomize_params()
	image_resized := create_image()
	png.Encode(w, image_resized)

}

func randomize_params() {
	radius = RADIUS_MIN + rand.Float64()*(RADIUS_MAX-RADIUS_MIN)
	// radius = rand.Float64() * RADIUS_MAX
	xpos = rand.Float64()
	ypos = rand.Float64()
}

func main() {

	if server_mode {

		http.HandleFunc("/", handler)
		log.Printf("Serving fractals on port %v\n", port)
		addr := fmt.Sprintf("0.0.0.0:%s", port)
		log.Fatal(http.ListenAndServe(addr, nil))

	} else {
		image_resized := create_image()
		out_file, _ := os.Create(out_filename)
		png.Encode(out_file, image_resized)
		fmt.Println("Finished writing to:", out_filename)
		// fmt.Printf("--r %v --x %v --y %v\n", radius, (xmin+xmax)/2, (ymin+ymax)/2)

	}
}
