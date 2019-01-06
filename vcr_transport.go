package govcr

import (
	"net/http"
)

// vcrTransport is the heart of VCR. It provides
// an http.RoundTripper that wraps over the default
// one provided by Go's http package or a custom one
// if specified when calling NewVCR.
type vcrTransport struct {
	PCB      *pcb
	Cassette *cassette
}

// RoundTrip is an implementation of http.RoundTripper.
func (t *vcrTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Note: by convention resp should be nil if an error occurs with HTTP
	var resp *http.Response

	// copy the request before the body is closed by the HTTP server.
	copiedReq, err := copyRequest(req)
	if err != nil {
		t.PCB.Logger.Println(err)
		return nil, err
	}

	request, err := newRequest(req, t.PCB.Logger)
	if err != nil {
		return nil, err
	}
	request = t.PCB.RequestFilter(request)

	// attempt to use a track from the cassette that matches
	// the request if one exists.
	if trackNumber := t.PCB.seekTrack(t.Cassette, request); trackNumber != trackNotFound {
		// Only the played back response is filtered.
		// The live request and response should NOT EVER be changed!
		request, _ = newRequest(req, t.PCB.Logger)
		resp = t.PCB.filterResponse(t.Cassette.replayResponse(trackNumber, copiedReq), request)

		return resp, err
	}

	// no recorded track was found so execute the request live
	t.PCB.Logger.Printf("INFO - Cassette '%s' - Executing request to live server for %s %s\n", t.Cassette.Name, req.Method, req.URL.String())

	resp, err = t.PCB.Transport.RoundTrip(req)

	if !t.PCB.DisableRecording {
		// the VCR is not in read-only mode so
		// record the HTTP traffic into a new track on the cassette
		t.PCB.Logger.Printf("INFO - Cassette '%s' - Recording new track for %s %s as %s %s\n", t.Cassette.Name, req.Method, req.URL.String(), copiedReq.Method, copiedReq.URL)
		if err := recordNewTrackToCassette(t.Cassette, copiedReq, resp, err); err != nil {
			t.PCB.Logger.Println(err)
		}
	}

	return resp, err
}
