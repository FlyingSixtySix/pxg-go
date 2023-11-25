package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"github.com/gin-gonic/gin"
	"image"
	"image/color"
	"image/png"
	"log"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	_ "github.com/joho/godotenv/autoload"
)

type Config struct {
	Width             int      `json:"width"`
	Height            int      `json:"height"`
	DefaultColorIndex int      `json:"defaultColorIndex"`
	Palette           []string `json:"palette"`
}

type ClientPixel struct {
	X     int `json:"x"`
	Y     int `json:"y"`
	Color int `json:"color"`
}

type ServerPixel struct {
	X     int   `json:"x"`
	Y     int   `json:"y"`
	Color int   `json:"color"`
	Time  int64 `json:"time"`
}

var config *Config
var board []byte
var placementData []ServerPixel
var saveTicker *time.Ticker

func main() {
	log.Printf("PxG v0.1.0")
	width, _ := strconv.Atoi(os.Getenv("BOARD_WIDTH"))
	height, _ := strconv.Atoi(os.Getenv("BOARD_HEIGHT"))
	expectedSize := width * height
	defaultColorIndex, _ := strconv.Atoi(os.Getenv("DEFAULT_COLOR_INDEX"))
	palette := strings.Split(os.Getenv("PALETTE"), ",")
	config = &Config{
		Width:             width,
		Height:            height,
		DefaultColorIndex: defaultColorIndex,
		Palette:           palette,
	}

	loadCanvas("storage/board.dat", expectedSize, defaultColorIndex)
	loadPlacementData("storage/data.json")

	saveTicker = time.NewTicker(10 * time.Second)
	go func() {
		for {
			select {
			case <-saveTicker.C:
				saveCanvas("storage/board.dat")
				savePlacementData("storage/data.json")
			}
		}
	}()

	r := gin.Default()
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "pong",
		})
	})
	r.GET("/info", func(c *gin.Context) {
		c.JSON(200, config)
	})
	r.GET("/board", func(c *gin.Context) {
		c.Data(200, "application/octet-stream", board)
	})
	r.GET("/pixel", func(c *gin.Context) {
		var xQuery string
		var yQuery string
		var exists bool
		xQuery, exists = c.GetQuery("x")
		yQuery, exists = c.GetQuery("y")
		if !exists {
			c.Status(400)
		}
		x, err := strconv.Atoi(xQuery)
		if err != nil {
			badRequest(c, "x is not a number")
		}
		y, err := strconv.Atoi(yQuery)
		if err != nil {
			badRequest(c, "y is not a number")
		}
		pixelIndex := slices.IndexFunc(placementData, func(pixel ServerPixel) bool {
			return pixel.X == x && pixel.Y == y
		})
		if pixelIndex == -1 {
			c.Status(404)
		}
		c.JSON(200, placementData[pixelIndex])
	})
	r.POST("/pixel", func(c *gin.Context) {
		var clientPlace ClientPixel
		if err := c.BindJSON(&clientPlace); err != nil {
			log.Println(err.Error())
			badRequest(c, "malformed body")
			return
		}
		if clientPlace.Color < 0 || clientPlace.Color > 255 {
			badRequest(c, "color out of range")
			return
		}
		if clientPlace.X < 0 || clientPlace.X >= width {
			badRequest(c, "x-coordinate out of range")
			return
		}
		if clientPlace.Y < 0 || clientPlace.Y >= height {
			badRequest(c, "y-coordinate out of range")
			return
		}
		board[clientPlace.Y*width+clientPlace.X] = byte(clientPlace.Color)
		now := time.Now().UnixMilli()
		placementData = append(placementData, ServerPixel{
			X:     clientPlace.X,
			Y:     clientPlace.Y,
			Color: clientPlace.Color,
			Time:  now,
		})
		c.Status(200)
	})
	r.GET("/image", func(c *gin.Context) {
		convertedPalette := paletteRGBA(palette)
		img := image.NewRGBA(image.Rect(0, 0, width, height))
		for y := 0; y < height; y++ {
			for x := 0; x < width; x++ {
				if x == 0 && y == 0 {
					log.Println(convertedPalette[board[y*width+x]])
				}
				img.Set(x, y, convertedPalette[board[y*width+x]])
			}
		}
		var buf bytes.Buffer
		err := png.Encode(&buf, img)
		if err != nil {
			log.Println(err.Error())
		}
		c.Data(200, "image/png", buf.Bytes())
	})
	_ = r.SetTrustedProxies(nil)
	_ = r.Run("localhost:8080")
}

func badRequest(c *gin.Context, message string) {
	c.JSON(400, gin.H{
		"message": message,
	})
}

func loadCanvas(path string, expectedSize int, defaultColor int) {
	board = make([]byte, expectedSize)
	fileInfo, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			for i := range board {
				board[i] = byte(defaultColor)
			}
		} else {
			log.Fatalln(err.Error())
		}
		return
	}
	if fileInfo.Size() != int64(expectedSize) {
		log.Fatalf("board size (%d) did not match expected size (%d)\n", fileInfo.Size(), expectedSize)
	}
	file, err := os.Open(path)
	if err != nil {
		log.Fatalln(err.Error())
	}
	_, err = file.Read(board)
	if err != nil {
		log.Fatalln(err.Error())
	}
}

func saveCanvas(path string) {
	err := os.WriteFile(path, board, 0644)
	if err != nil {
		log.Fatalln(err.Error())
	}
}

func loadPlacementData(path string) {
	fileInfo, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			placementData = make([]ServerPixel, 0)
		} else {
			log.Fatalln(err.Error())
		}
		return
	}
	file, err := os.Open(path)
	if err != nil {
		log.Fatalln(err.Error())
	}
	fileBytes := make([]byte, fileInfo.Size())
	_, err = file.Read(fileBytes)
	if err != nil {
		log.Fatalln(err.Error())
	}
	if err = json.Unmarshal(fileBytes, &placementData); err != nil {
		log.Fatalln(err.Error())
	}
}

func savePlacementData(path string) {
	placementBytes, err := json.Marshal(placementData)
	if err != nil {
		log.Fatalln(err.Error())
	}
	err = os.WriteFile(path, placementBytes, 0644)
	if err != nil {
		log.Fatalln(err.Error())
	}
}

func paletteRGBA(palette []string) []color.RGBA {
	converted := make([]color.RGBA, len(palette))
	for i := 0; i < len(palette); i++ {
		values, err := strconv.ParseUint(palette[i], 16, 32)
		if err != nil {
			log.Fatalln(err)
		}
		converted[i] = color.RGBA{
			R: uint8(values >> 16),
			G: uint8(values >> 8 & 0xFF),
			B: uint8(values & 0xFF),
			A: 0xFF,
		}
	}
	return converted
}
