package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/NebulousLabs/Sia/types"
)

// A structure to store any state of the server. Should remain relatively
// unpopulated, mostly constants which will eventually be broken off
type ExploreServer struct {
	url      string
	serveMux *http.ServeMux
}

// writeJSON writes the object to the ResponseWriter. If the encoding fails,
// an error is written instead.
func writeJSON(w http.ResponseWriter, obj interface{}) {
	if json.NewEncoder(w).Encode(obj) != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// overviewPage handles the default request, which displays a summary of the
// blockchain
func (es *ExploreServer) overviewPage(w http.ResponseWriter, r *http.Request) {
	// First query the local instance of siad for the status
	explorerState, err := es.apiExplorerState()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	blocklist, err := es.apiGetBlockData(0, explorerState.Height)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	nv, err := es.apiGet("/daemon/version")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	var nvs string
	err = json.Unmarshal(nv, &nvs)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Attempt to make a page out of it
	page, err := es.parseTemplate("overview.html", overviewRoot{
		Explorer:       explorerState,
		BlockSummaries: blocklist,
		NodeVersion:    nvs,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Write(page)
}

// heightHandler handles the request to get a block by block height by
// redirecting the request to the relevant block ID
func (es *ExploreServer) heightHandler(w http.ResponseWriter, r *http.Request) {
	// Extract the height
	var height types.BlockHeight
	_, err := fmt.Sscanf(r.FormValue("h"), "%d", &height)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Request info on that height
	blockSummaries, err := es.apiGetBlockData(height, height+1)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("hash?h=%s", blockSummaries[0].ID), 301)
}

// rootHandler handles the root page being requested. Is responsible for
// differentiating between API calls and pages
func (es *ExploreServer) rootHandler(w http.ResponseWriter, r *http.Request) {
	if strings.Contains(r.URL.Path, ".") {
		es.serveMux.ServeHTTP(w, r)
	} else {
		es.overviewPage(w, r)
	}
}

func main() {
	// Parse command line flags for port numbers
	apiPort := flag.String("a", "9980", "API port")
	hostPort := flag.String("p", "9983", "HTTP host port")
	flag.Parse()

	// Initialize the server
	var es = &ExploreServer{
		url:      "http://localhost:" + *apiPort,
		serveMux: http.NewServeMux(),
	}

	es.serveMux.Handle("/", http.FileServer(http.Dir("./webroot/")))
	http.HandleFunc("/", es.rootHandler)
	http.HandleFunc("/hash", es.hashPageHandler)
	http.HandleFunc("/height", es.heightHandler)
	http.HandleFunc("/hosts", es.hostsHandler)
	err := http.ListenAndServe(":"+*hostPort, nil)
	if err != nil {
		fmt.Println("Error when serving:", err)
		os.Exit(1)
	}
	fmt.Println("Done serving")
}
