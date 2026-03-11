package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/dchill72/jetski"
)

func main() {
	width := flag.Int("width", jetski.DefaultWidth, "output column width")
	brightness := flag.Float64("brightness", 0.5, "overall brightness [0.0 dark → 0.5 neutral → 1.0 light]")
	gamma := flag.Float64("gamma", 0, "luminance gamma (<1 lighter, >1 darker, 0=off)")
	equalize := flag.Bool("equalize", false, "apply histogram equalization")
	color := flag.Bool("color", false, "apply RGB wave gradient (ANSI 24-bit color)")
	angle := flag.Float64("angle", 45, "wave angle in degrees (0=horizontal, 90=vertical)")
	phase := flag.Float64("phase", 0, "wave phase offset in radians")
	period := flag.Float64("period", 16, "wave period in character cells")
	saturation := flag.Float64("saturation", 1, "color saturation [0-1]")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: %s [flags] <image>\n\nflags:\n", os.Args[0])
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nexample images in src/images/\n")
	}
	flag.Parse()

	if flag.NArg() < 1 {
		flag.Usage()
		os.Exit(1)
	}

	f, err := os.Open(flag.Arg(0))
	if err != nil {
		fmt.Fprintf(os.Stderr, "open: %v\n", err)
		os.Exit(1)
	}
	defer f.Close()

	jOpts := jetski.Options{
		Width:      *width,
		Brightness: *brightness,
		Gamma:      *gamma,
		Equalize:   *equalize,
	}

	if !*color {
		result, err := jetski.Convert(f, jOpts)
		if err != nil {
			fmt.Fprintf(os.Stderr, "convert: %v\n", err)
			os.Exit(1)
		}
		fmt.Print(result)
		return
	}

	grid, err := jetski.ConvertGrid(f, jOpts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "convert: %v\n", err)
		os.Exit(1)
	}
	fmt.Print(jetski.Colorize(grid, jetski.WaveOptions{
		Angle:  *angle,
		Phase:  *phase,
		Period: *period,
		Range:  *saturation,
	}))
}
