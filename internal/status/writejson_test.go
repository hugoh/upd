package status

import (
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWriteJSON_OmitsZeroLastSuccess(t *testing.T) {
	tests := []struct {
		name     string
		data     any
		contains []string
		absent   []string
	}{
		{
			name:     "zero lastSuccess is omitted",
			data:     Report{Loop: &LoopStatus{}},
			contains: []string{`"interval"`},
			absent:   []string{`"lastSuccess"`},
		},
		{
			name:     "non-zero lastSuccess is kept",
			data:     Report{Loop: &LoopStatus{LastSuccess: ReadableDuration(5e9)}},
			contains: []string{`"lastSuccess": "5s"`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			writeJSON(rec, tt.data)

			body := rec.Body.String()
			for _, s := range tt.contains {
				require.Contains(t, body, s)
			}

			for _, s := range tt.absent {
				require.NotContains(t, body, s)
			}
		})
	}
}
