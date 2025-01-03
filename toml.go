package structconf

import (
	"fmt"
	"os"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/urfave/cli/v3"
)

// workaround until https://github.com/urfave/cli/issues/2037 and https://github.com/urfave/cli-altsrc/issues/14
// are fixed

type mapSource struct {
	name string
	m    map[any]any
}

func NewMapSource(name string, m map[any]any) cli.MapSource {
	return &mapSource{
		name: name,
		m:    m,
	}
}

func NewTomlFileSource(name string, file string) (cli.MapSource, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %q: %w", file, err)
	}

	container := make(map[any]any)
	if err := toml.Unmarshal(data, &container); err != nil {
		return nil, fmt.Errorf("failed to parse file as toml %q: %w", file, err)
	}

	return NewMapSource(name, container), nil
}

func (ms *mapSource) String() string { return fmt.Sprintf("map source %[1]q", ms.name) }
func (ms *mapSource) GoString() string {
	return fmt.Sprintf("&mapSource{flag:%[1]q}", ms.name)
}

func (ms *mapSource) Lookup(name string) (any, bool) {
	node := ms.m
	sections := strings.Split(name, ".")

	if len(sections) == 0 {
		return nil, false
	}

	// recurse into the map, splitting the key on "."
	if len(sections) > 1 {
		for _, section := range sections[:len(sections)-1] {
			child, ok := node[section]
			if !ok {
				return nil, false
			}

			switch child := child.(type) {
			case map[string]any:
				node = make(map[any]any, len(child))
				for k, v := range child {
					node[k] = v
				}
			case map[any]any:
				node = child
			default:
				return nil, false
			}
		}
	}

	// now lookup the last section
	if val, ok := node[sections[len(sections)-1]]; ok {
		return val, true
	}

	return nil, false
}

type mapsValueSource struct {
	key  string
	maps []cli.MapSource
}

func (mvs *mapsValueSource) String() string {
	return fmt.Sprintf("key %[1]q from %[2]d maps", mvs.key, len(mvs.maps))
}

func (mvs *mapsValueSource) GoString() string {
	return fmt.Sprintf("&mapsValueSource{key:%[1]q, src:%[2]v}", mvs.key, mvs.maps)
}

func (mvs *mapsValueSource) Lookup() (string, bool) {
	for _, ms := range mvs.maps {
		if v, ok := ms.Lookup(mvs.key); ok { // return the first defaultValue found
			return fmt.Sprintf("%+v", v), true
		}
	}
	return "", false
}

func NewValueSourceFromMaps(key string, sources ...cli.MapSource) cli.ValueSource {
	return &mapsValueSource{
		key:  key,
		maps: sources,
	}
}
