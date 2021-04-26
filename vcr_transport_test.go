package govcr

import (
	"log"
	"net/http"
	"os"
	"reflect"
	"strings"
	"testing"
)

type mockRoundTripper struct {
	statusCode int
}

func (t *mockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.statusCode == 0 {
		t.statusCode = http.StatusMovedPermanently
	}
	return &http.Response{
		Request:    req,
		StatusCode: t.statusCode,
	}, nil
}

func Test_vcrTransport_RoundTrip_doesNotChangeLiveReqOrLiveResp(t *testing.T) {
	logger := log.New(os.Stderr, "", log.LstdFlags)
	out, err := os.OpenFile(os.DevNull, os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		t.Errorf("unable to initialise logger - error = %v", err)
		return
	}
	defer func() { out.Close() }()
	logger.SetOutput(out)

	mutateReq := RequestFilter(func(req Request) Request {
		req.Method = "INVALID"
		req.URL.Host = "host.changed.internal"
		return req
	})
	requestFilters := RequestFilters{}
	requestFilters.Add(mutateReq)

	mutateResp := ResponseFilter(func(resp Response) Response {
		resp.StatusCode = -9999
		return resp
	})
	responseFilters := ResponseFilters{}
	responseFilters.Add(mutateResp)

	mrt := &mockRoundTripper{}
	transport := &vcrTransport{
		PCB: &pcb{
			DisableRecording: true,
			Transport:        mrt,
			RequestFilter:    requestFilters.combined(),
			ResponseFilter:   responseFilters.combined(),
			Logger:           logger,
			CassettePath:     "",
		},
		Cassette: newCassette("", ""),
	}

	req, err := http.NewRequest("GET", "https://example.com/path?query", toReadCloser([]byte("Lorem ipsum dolor sit amet")))
	if err != nil {
		t.Errorf("req http.NewRequest() error = %v", err)
		return
	}

	wantReq, err := http.NewRequest("GET", "https://example.com/path?query", toReadCloser([]byte("Lorem ipsum dolor sit amet")))
	if err != nil {
		t.Errorf("wantReq http.NewRequest() error = %v", err)
		return
	}

	gotResp, err := transport.RoundTrip(req)
	if err != nil {
		t.Errorf("vcrTransport.RoundTrip() error = %v", err)
		return
	}
	wantResp := http.Response{
		Request:    wantReq,
		StatusCode: http.StatusMovedPermanently,
	}

	if !reflect.DeepEqual(req, wantReq) {
		t.Errorf("vcrTransport.RoundTrip() Request has been modified = %+v, want %+v", req, wantReq)
	}

	if !reflect.DeepEqual(gotResp, &wantResp) {
		t.Errorf("vcrTransport.RoundTrip() Response has been modified = %+v, want %+v", gotResp, wantResp)
	}
}

// Test checks that live request was not executed for track not presented in cassette
func TestVcrTransport_RoundTrip_NoLiveConnections(t *testing.T) {
	tr := &mockRoundTripper{
		statusCode: http.StatusTeapot,
	}
	// any cassette
	vcr := NewVCR("MyCassette1", &VCRConfig{
		Logging:           false,
		NoLiveConnections: true,
		//  without DisableRecording flag nonexisting request will be written to cassette and test will fail next time
		DisableRecording: true,
		Client:           &http.Client{Transport: tr},
	})
	// any request, not mentioned in cassette
	resp, err := vcr.Client.Get("http://www.no_such_cassete.com/nonexisting")

	if resp != nil && resp.StatusCode == tr.statusCode {
		t.Fatalf("Live request was executed")
	}
	if err == nil {
		t.Fatal("error expected here")
	}
	if !strings.Contains(err.Error(), ErrNoTrackFound.Error()) {
		t.Errorf("expected err %v, actual %v", ErrNoTrackFound, err)
	}
}
