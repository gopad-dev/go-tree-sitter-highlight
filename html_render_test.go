package highlight

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

var cssTheme = map[string]string{
	"variable": "color: #DEAE60;",
	"function": "color: #73FBF1;",
	"string":   "color: #B8E466;",
	"keyword":  "color: #A578EA;",
	"comment":  "color: #8A8A8A;",
}

func TestHTMLRender_Render(t *testing.T) {
	captureNames := make([]string, 0, len(theme))
	for name := range theme {
		captureNames = append(captureNames, name)
	}

	source, err := os.ReadFile("testdata/test.go")
	require.NoError(t, err)

	cfg := loadInjection(t, captureNames)("go")

	highlighter := New()

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	events := highlighter.Highlight(ctx, *cfg, source, func(name string) *Configuration {
		return nil
	})

	f, err := os.Create("out.html")
	require.NoError(t, err)
	defer func() {
		err = f.Close()
		require.NoError(t, err)
	}()

	htmlRender := NewHTMLRender()
	err = htmlRender.RenderDocument(f, events, "test.go", source, captureNames, cssTheme)
	require.NoError(t, err)
}
