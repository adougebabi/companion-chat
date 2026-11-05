package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
)

const defaultImageResolution = "768"

var imageAspectRatios = map[string][2]int64{
	"1:1": {1, 1}, "3:2": {3, 2}, "2:3": {2, 3}, "16:9": {16, 9}, "9:16": {9, 16},
}

func validImageResolution(value string) bool {
	switch value {
	case "512", "768", "1024", "1280", "1536", "2048":
		return true
	default:
		return false
	}
}

// imageDimensions freezes a node's ratio and resolution separately. Width and
// height are derived only for workflows that use numeric dimension inputs.
func imageDimensions(ratio string) (map[string]any, error) {
	return imageDimensionsAtResolution(ratio, defaultImageResolution)
}

func imageDimensionsAtResolution(ratio, resolution string) (map[string]any, error) {
	pair, ok := imageAspectRatios[ratio]
	if !ok {
		return nil, errors.New("media_aspect_ratio_invalid")
	}
	if !validImageResolution(resolution) {
		return nil, errors.New("media_resolution_invalid")
	}
	level, _ := strconv.Atoi(resolution)
	longSide := max(pair[0], pair[1])
	steps := int64(math.Round(float64(level) / float64(longSide*8)))
	return map[string]any{"aspect_ratio": ratio, "width": pair[0] * steps * 8, "height": pair[1] * steps * 8, "resolution": resolution}, nil
}

func requestedImageDimensions(args map[string]any) (map[string]any, error) {
	ratio := "9:16"
	if raw, ok := args["aspect_ratio"]; ok {
		var valid bool
		ratio, valid = raw.(string)
		if _, supported := imageAspectRatios[ratio]; !valid || !supported {
			return nil, errors.New("media_aspect_ratio_invalid")
		}
	}
	resolution := defaultImageResolution
	if raw, ok := args["resolution"]; ok {
		var valid bool
		resolution, valid = raw.(string)
		if !valid || !validImageResolution(resolution) {
			return nil, errors.New("media_resolution_invalid")
		}
	}
	return imageDimensionsAtResolution(ratio, resolution)
}

func mediaImageDimensions(concept map[string]any) (map[string]any, error) {
	if raw, exists := concept["image_dimensions"]; exists {
		dimensions, ok := raw.(map[string]any)
		if !ok {
			return nil, errors.New("media_image_dimensions_invalid")
		}
		ratio := stringValue(dimensions["aspect_ratio"])
		width, widthOK := imageDimensionInt(dimensions["width"])
		height, heightOK := imageDimensionInt(dimensions["height"])
		pair, ratioOK := imageAspectRatios[ratio]
		if ratio == "3:4" { // A previously frozen character-card ratio.
			pair, ratioOK = [2]int64{3, 4}, true
		}
		resolution := stringValue(dimensions["resolution"])
		if !ratioOK || !widthOK || !heightOK || width*pair[1] != height*pair[0] {
			return nil, errors.New("media_image_dimensions_invalid")
		}
		if validImageResolution(resolution) {
			expected, err := imageDimensionsAtResolution(ratio, resolution)
			if err != nil || expected["width"] != width || expected["height"] != height {
				return nil, errors.New("media_image_dimensions_invalid")
			}
		} else if resolution != fmt.Sprintf("%dx%d", width, height) {
			return nil, errors.New("media_image_dimensions_invalid")
		}
		if concept["purpose"] == "visual_identity" && ((concept["stage"] == "avatar" && ratio != "1:1") || (concept["stage"] != "avatar" && ratio != "2:3" && ratio != "3:4")) {
			return nil, errors.New("media_image_dimensions_conflict")
		}
		return map[string]any{"aspect_ratio": ratio, "width": width, "height": height, "resolution": dimensions["resolution"]}, nil
	}
	if concept["purpose"] == "visual_identity" {
		if concept["stage"] == "avatar" {
			return legacyImageDimensions("1:1"), nil
		}
		return legacyImageDimensions("3:4"), nil
	}
	return legacyImageDimensions("9:16"), nil
}

// Preset-node pixel geometry is node-owned, so model input carries only the
// chosen ratio and resolution, not the fallback width/height for other graphs.
func mediaImageDimensionsForModel(concept map[string]any) (map[string]any, error) {
	dimensions, err := mediaImageDimensions(concept)
	if err != nil {
		return nil, err
	}
	return map[string]any{"aspect_ratio": dimensions["aspect_ratio"], "resolution": dimensions["resolution"]}, nil
}

// Missing and pre-preset snapshots keep their original dimensions on retry.
func legacyImageDimensions(ratio string) map[string]any {
	width, height := 720, 1280
	switch ratio {
	case "1:1":
		width, height = 1024, 1024
	case "3:4":
		width, height = 768, 1024
	}
	return map[string]any{"aspect_ratio": ratio, "width": width, "height": height, "resolution": fmt.Sprintf("%dx%d", width, height)}
}

func imageDimensionInt(raw any) (int64, bool) {
	var value int64
	switch v := raw.(type) {
	case int:
		value = int64(v)
	case int64:
		value = v
	case float64:
		if math.Trunc(v) != v || v < 1 || v > 32768 {
			return 0, false
		}
		value = int64(v)
	case json.Number:
		var err error
		value, err = v.Int64()
		if err != nil {
			return 0, false
		}
	default:
		return 0, false
	}
	return value, value > 0 && value <= 32768 && value%8 == 0
}

// Exact width/height placeholders remain JSON numbers for ComfyUI inputs;
// embedded placeholders and the ratio/resolution values are strings.
func replaceMediaDimensionPlaceholders(value string, dimensions map[string]any) (any, bool, error) {
	fields := [][2]string{
		{"{{width}}", "width"}, {"{{height}}", "height"},
		{"{{ratio}}", "aspect_ratio"}, {"{{aspect_ratio}}", "aspect_ratio"}, {"{{resolution}}", "resolution"},
	}
	result := value
	matched := false
	for _, field := range fields {
		if !strings.Contains(result, field[0]) {
			continue
		}
		matched = true
		replacement, exists := dimensions[field[1]]
		if !exists || replacement == nil {
			return nil, true, errors.New("media_image_dimensions_missing")
		}
		if value == field[0] {
			return replacement, true, nil
		}
		result = strings.ReplaceAll(result, field[0], fmt.Sprint(replacement))
	}
	return result, matched, nil
}
