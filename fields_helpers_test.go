package hyperlogger

import (
	"reflect"
	"testing"
	"time"

	"github.com/hyp3rd/ewrap"
)

func TestFieldHelpers(t *testing.T) {
	const (
		countValue = 5
		sizeValue  = uint64(3)
		totalValue = int64(9)
		ratioValue = 1.5
		errorKey   = "error"
	)

	errExample := ewrap.New("boom")
	now := time.Now()

	tests := []struct {
		name    string
		field   Field
		wantKey string
		want    any
	}{
		{"Str", Str("k", "v"), "k", "v"},
		{"Bool", Bool("flag", true), "flag", true},
		{"Int", Int("count", countValue), "count", countValue},
		{"Int64", Int64("total", totalValue), "total", totalValue},
		{"Uint64", Uint64("size", sizeValue), "size", sizeValue},
		{"Float64", Float64("ratio", ratioValue), "ratio", ratioValue},
		{"Duration", Duration("latency", time.Second), "latency", time.Second},
		{"Time", Time("ts", now), "ts", now},
		{"Error", Error(errorKey, errExample), errorKey, errExample},
		{"ErrorNil", Error(errorKey, nil), errorKey, nil},
		{"Any", Any("custom", []string{"a"}), "custom", []string{"a"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.field.Key != tt.wantKey {
				t.Fatalf("expected key %s, got %s", tt.wantKey, tt.field.Key)
			}

			if !reflect.DeepEqual(tt.field.Value, tt.want) {
				t.Fatalf("expected value %#v, got %#v", tt.want, tt.field.Value)
			}
		})
	}
}
