package endpoint

import (
	"fmt"
	"net/http"
)

func NewEndpoint() {

}

func handler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Hi there, I love %s!", r.URL.Path[1:])
}