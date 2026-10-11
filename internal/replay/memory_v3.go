package replay

import "fmt"

const (
	DefaultReplayMemoryV3 int64 = 256 << 20
	MaxReplayMemoryV3     int64 = 2 << 30
)

func ParseReplayMemoryV3(value string) (int64, error) {
	switch value {
	case "256m":
		return 256 << 20, nil
	case "512m":
		return 512 << 20, nil
	case "1g":
		return 1 << 30, nil
	case "2g":
		return 2 << 30, nil
	default:
		return 0, fmt.Errorf(
			"replay: invalid v3 memory limit %q; use 256m, 512m, 1g, or 2g",
			value,
		)
	}
}
