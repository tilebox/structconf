package structconf

import (
	"fmt"
	"os"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/urfave/cli/v3"
)

func NewMapSource(name string, m map[any]any) cli.MapSource {
	return cli.NewMapSource(name, m)
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
			return formatMapSourceValue(v), true
		}
	}

	return "", false
}

func formatMapSourceValue(value any) string {
	switch v := value.(type) {
	case []string:
		return strings.Join(v, ",")
	case []any:
		values := make([]string, 0, len(v))
		for _, item := range v {
			values = append(values, fmt.Sprintf("%+v", item))
		}
		return strings.Join(values, ",")
	default:
		return fmt.Sprintf("%+v", value)
	}
}

func NewValueSourceFromMaps(key string, sources ...cli.MapSource) cli.ValueSource {
	return &mapsValueSource{
		key:  key,
		maps: sources,
	}
}
