package hyperlogger

import (
	"reflect"
	"testing"
	"time"

	"github.com/hyp3rd/ewrap"
)

func TestFieldHelpers(t *testing.T) {
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
		{"Int", Int("count", 5), "count", 5},
		{"Int64", Int64("total", int64(9)), "total", int64(9)},
		{"Uint64", Uint64("size", uint64(3)), "size", uint64(3)},
		{"Float64", Float64("ratio", 1.5), "ratio", 1.5},
		{"Duration", Duration("latency", time.Second), "latency", time.Second},
		{"Time", Time("ts", now), "ts", now},
		{"Error", Error("error", errExample), "error", errExample},
		{"ErrorNil", Error("error", nil), "error", nil},
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
