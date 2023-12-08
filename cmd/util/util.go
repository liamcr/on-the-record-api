package util

import (
	"fmt"
	"image"
	_ "image/jpeg"
	"math"
	"net/http"
	"sort"
)

type Pixel struct {
	R int
	G int
	B int
}

func rgbaToPixel(r uint32, g uint32, b uint32, a uint32) Pixel {
	return Pixel{
		R: int(r / 257),
		G: int(g / 257),
		B: int(b / 257),
	}
}

// Get the bi-dimensional pixel array
func getPixels(img image.Image) []Pixel {
	bounds := img.Bounds()
	width, height := bounds.Max.X, bounds.Max.Y

	var pixels []Pixel
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			pixels = append(pixels, rgbaToPixel(img.At(x, y).RGBA()))
		}
	}

	return pixels
}

func findBiggestColorRange(rgbValues []Pixel) string {
	rMin := math.MaxInt
	gMin := math.MaxInt
	bMin := math.MaxInt

	rMax := math.MinInt
	gMax := math.MinInt
	bMax := math.MinInt

	for _, pixel := range rgbValues {
		rMin = min(rMin, pixel.R)
		gMin = min(gMin, pixel.G)
		bMin = min(bMin, pixel.B)

		rMax = max(rMax, pixel.R)
		gMax = max(gMax, pixel.G)
		bMax = max(bMax, pixel.B)
	}

	rRange := rMax - rMin
	gRange := gMax - gMin
	bRange := bMax - bMin

	biggestRange := max(rRange, max(gRange, bRange))
	if biggestRange == rRange {
		return "R"
	} else if biggestRange == gRange {
		return "G"
	} else {
		return "B"
	}
}

const MAX_DEPTH = 1

func quantization(rgbValues []Pixel, depth int) []Pixel {
	if depth == MAX_DEPTH || len(rgbValues) == 0 {
		color := Pixel{0, 0, 0}

		for _, rgb := range rgbValues {
			color.R += rgb.R
			color.G += rgb.G
			color.B += rgb.B
		}

		color.R = color.R / len(rgbValues)
		color.G = color.G / len(rgbValues)
		color.B = color.B / len(rgbValues)

		return []Pixel{color}
	}

	componentToSortBy := findBiggestColorRange(rgbValues)
	sort.Slice(rgbValues, func(i, j int) bool {
		if componentToSortBy == "R" {
			return rgbValues[i].R > rgbValues[j].R
		}
		if componentToSortBy == "G" {
			return rgbValues[i].G > rgbValues[j].G
		}
		if componentToSortBy == "B" {
			return rgbValues[i].B > rgbValues[j].B
		}
		return true
	})

	mid := len(rgbValues) / 2
	return append(quantization(rgbValues[:mid], depth+1), quantization(rgbValues[mid+1:], depth+1)...)
}

func GetDominantColourFromImage(url string) (string, error) {
	response, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()

	img, _, err := image.Decode(response.Body)
	if err != nil {
		return "", err
	}

	pixels := getPixels(img)

	dominantColours := quantization(pixels, 0)
	return fmt.Sprintf("rgba(%d, %d, %d, 0.8)", dominantColours[0].R, dominantColours[0].G, dominantColours[0].B), nil
}
