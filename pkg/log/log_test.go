package log

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	logger "github.com/hyp3rd/hyperlogger"
	"github.com/hyp3rd/hyperlogger/internal/constants"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name        string
		environment string
		service     string
		wantLevel   logger.Level
		wantJSON    bool
		wantErr     bool
	}{
		{
			name:        "non-production environment",
			environment: constants.NonProductionEnvironment,
			service:     "test-service",
			wantLevel:   logger.DebugLevel,
			wantJSON:    false,
			wantErr:     false,
		},
		{
			name:        "production environment",
			environment: "production",
			service:     "test-service",
			wantLevel:   logger.InfoLevel,
			wantJSON:    true,
			wantErr:     false,
		},
		{
			name:        "empty environment",
			environment: "",
			service:     "test-service",
			wantLevel:   logger.InfoLevel,
			wantJSON:    true,
			wantErr:     false,
		},
		{
			name:        "empty service name",
			environment: constants.NonProductionEnvironment,
			service:     "",
			wantLevel:   logger.DebugLevel,
			wantJSON:    false,
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			log, err := New(context.Background(), tt.environment, tt.service)
			if tt.wantErr {
				require.Error(t, err)
				assert.Nil(t, log)

				return
			}

			require.NoError(t, err)
			require.NotNil(t, log)

			config := log.GetConfig()
			assert.Equal(t, tt.wantLevel, config.Level)
			assert.Equal(t, tt.wantJSON, config.EnableJSON)
			assert.True(t, config.EnableCaller)

			// Verify additional fields
			var foundService, foundEnv bool

			for _, field := range config.AdditionalFields {
				switch field.Key {
				case "service":
					assert.Equal(t, tt.service, field.Value)

					foundService = true
				case "environment":
					assert.Equal(t, tt.environment, field.Value)

					foundEnv = true
				}
			}

			assert.True(t, foundService, "service field should be present")
			assert.True(t, foundEnv, "environment field should be present")
		})
	}
}
