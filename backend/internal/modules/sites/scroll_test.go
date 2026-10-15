package sites

import (
	"bytes"
	"testing"
)

func TestInjectScrollScriptOnce(t *testing.T) {
	first, changed := InjectScrollScript([]byte("<html><body>Бородино</body></html>"))
	if !changed || !bytes.HasSuffix(first, []byte(scrollScript)) {
		t.Fatalf("first inject: changed = %v, html = %s", changed, first)
	}
	second, changed := InjectScrollScript(first)
	if changed || !bytes.Equal(first, second) {
		t.Fatalf("second inject changed html: %s", second)
	}
	if !IsHTML("about/Index.HTM") || IsHTML("style.css") {
		t.Fatal("IsHTML mismatch")
	}
}
