package hyperlogger

import (
	"context"
	"testing"
)

type testExtractorKey struct{}

func TestRegisterContextExtractor(t *testing.T) {
	ClearContextExtractors()
	t.Cleanup(ClearContextExtractors)

	RegisterContextExtractor(func(ctx context.Context) []Field {
		if v := ctx.Value(testExtractorKey{}); v != nil {
			return []Field{{Key: "key", Value: v}}
		}

		return nil
	})

	extractors := GlobalContextExtractors()
	if len(extractors) != 1 {
		t.Fatalf("expected 1 extractor, got %d", len(extractors))
	}

	fields := ApplyContextExtractors(context.WithValue(context.Background(), testExtractorKey{}, "value"), extractors...)
	if len(fields) != 1 || fields[0].Key != "key" || fields[0].Value != "value" {
		t.Fatalf("unexpected fields: %+v", fields)
	}
}
