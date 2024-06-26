package hal

import (
	"net/http"

	"github.com/HashCash-Consultants/go/support/render/httpjson"
)

// Render write data to w, after marshaling to json
func Render(w http.ResponseWriter, data interface{}) {
	httpjson.Render(w, data, httpjson.HALJSON)
}
