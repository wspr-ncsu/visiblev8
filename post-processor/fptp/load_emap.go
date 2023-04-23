package fptp

import (
	"encoding/json"
	"os"
)

type EMap struct {
	EntityPropertyMap map[string]*EntityProperty
}

type EntityProperty struct {
	DisplayName string  `json:"displayName"`
	Tracking    float64 `json:"tracking"`
}

func NewEMap() *EMap {
	return &EMap{
		EntityPropertyMap: make(map[string]*EntityProperty),
	}
}

func loadEMap() (*EMap, error) {
	emap := NewEMap()

	emap_file := os.Getenv("EMAP_FILE")

	if emap_file == "" {
		emap_file = "./entities.json"
	}

	jsonBlob, err := os.ReadFile(emap_file)
	if err != nil {
		return nil, err
	}

	json.Unmarshal(jsonBlob, &emap.EntityPropertyMap)

	return emap, nil
}
