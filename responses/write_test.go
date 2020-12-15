package responses

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi"
	"github.com/tsaron/anansi/json"
	"syreclabs.com/go/faker"
)

func TestSend(t *testing.T) {
	router := chi.NewRouter()
	name := faker.Name().Name()

	router.Get("/json", func(w http.ResponseWriter, _ *http.Request) {
		b, _ := json.Marshal(map[string]string{
			"name": name,
		})

		Write(w, 200, b)
	})

	res := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/json", nil)
	router.ServeHTTP(res, req)

	resData := make(map[string]string)
	err := json.Unmarshal(res.Body.Bytes(), &resData)
	if err != nil {
		t.Fatal(err)
	}

	if resData["name"] != name {
		t.Errorf("Expected the name to be %s, got %s", name, resData["name"])
	}
}
