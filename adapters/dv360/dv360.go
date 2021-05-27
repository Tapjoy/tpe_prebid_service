package dv360

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/mxmCherry/openrtb"
	"github.com/prebid/prebid-server/adapters"
	"github.com/prebid/prebid-server/errortypes"
	"github.com/prebid/prebid-server/openrtb_ext"
	"github.com/prebid/prebid-server/pbs"
)

// SKAN IDs must be lower case
var dv360SKADNetIDs = map[string]bool{
	"fill_me_in.skadnetwork": true,
}

// Region ...
type Region string

const (
	USEast Region = "us_east"
)

type dv360ImpExt struct {
	SKADN *openrtb_ext.SKADN `json:"skadn,omitempty"`
}

type dv360VideoExt struct {
	Rewarded int `json:"rewarded"`
}

// DV360Adapter ...
type DV360Adapter struct {
	http             *adapters.HTTPAdapter
	URI              string
	SupportedRegions map[Region]string
}

// Name is used for cookies and such
func (adapter *DV360Adapter) Name() string {
	return "dv360"
}

// SkipNoCookies ...
func (adapter *DV360Adapter) SkipNoCookies() bool {
	return false
}

// Call is legacy, and added only to support DV360 interface
func (adapter *DV360Adapter) Call(_ context.Context, _ *pbs.PBSRequest, _ *pbs.PBSBidder) (pbs.PBSBidSlice, error) {
	return pbs.PBSBidSlice{}, nil
}

// NewDV360Adapter ...
func NewDV360Adapter(config *adapters.HTTPAdapterConfig, uri, useast string) *DV360Adapter {
	return NewDV360Bidder(adapters.NewHTTPAdapter(config).Client, uri, useast)
}

// NewDV360Bidder ...
func NewDV360Bidder(client *http.Client, uri, useast string) *DV360Adapter {
	return &DV360Adapter{
		http: &adapters.HTTPAdapter{Client: client},
		URI:  uri,
		SupportedRegions: map[Region]string{
			USEast: useast,
		},
	}
}

func (adapter *DV360Adapter) MakeRequests(request *openrtb.BidRequest, requestInfo *adapters.ExtraRequestInfo) ([]*adapters.RequestData, []error) {
	// number of requests
	numRequests := len(request.Imp)

	requestData := make([]*adapters.RequestData, 0, numRequests)

	// headers
	headers := http.Header{}
	headers.Add("Content-Type", "application/json")
	headers.Add("Accept", "application/json")
	headers.Add("User-Agent", "prebid-server/1.0")

	// errors
	errs := make([]error, 0, numRequests)

	// clone the request imp array
	requestImpCopy := request.Imp

	var err error

	for i := 0; i < numRequests; i++ {
		// clone current imp
		impCopy := requestImpCopy[i]

		// extract bidder extension
		var bidderExt adapters.ExtImpBidder
		if err = json.Unmarshal(impCopy.Ext, &bidderExt); err != nil {
			errs = append(errs, &errortypes.BadInput{
				Message: err.Error(),
			})
			continue
		}

		// unmarshal bidder extension to dv360 extension
		var dv360Ext openrtb_ext.ExtImpDV360
		if err = json.Unmarshal(bidderExt.Bidder, &dv360Ext); err != nil {
			errs = append(errs, &errortypes.BadInput{
				Message: err.Error(),
			})
			continue
		}

		rewarded := 0
		if dv360Ext.Reward == 1 {
			rewarded = 1
		}

		// if there is a banner object
		if impCopy.Banner != nil {
			// check if mraid is supported for this dsp
			if !dv360Ext.MRAIDSupported {
				// we don't support mraid, remove the banner object
				impCopy.Banner = nil
			}
		}

		// if we have a video object
		if impCopy.Video != nil {
			// make a copy of the video object
			videoCopy := *impCopy.Video

			// instantiate dv360 video extension
			videoExt := dv360VideoExt{
				Rewarded: rewarded,
			}

			// convert dv360 video extension to json
			// and append to copied video object
			videoCopy.Ext, err = json.Marshal(&videoExt)
			if err != nil {
				errs = append(errs, err)
				continue
			}

			// assign copied video object to copied impression object
			impCopy.Video = &videoCopy
		}

		// initial value
		skanSent := false

		// create impression extension object
		impExt := dv360ImpExt{}

		// check if skan is supported
		if dv360Ext.SKADNSupported {
			// get skan data
			skadn := adapters.FilterPrebidSKADNExt(bidderExt.Prebid, dv360SKADNetIDs)

			// if we have skan data
			if len(skadn.SKADNetIDs) > 0 {
				// set to true
				skanSent = true

				// apply skan data to impression extension object
				impExt.SKADN = &skadn
			}
		}

		// json marshal the impression extension and apply to
		// copied impression object
		impCopy.Ext, err = json.Marshal(&impExt)
		if err != nil {
			errs = append(errs, err)
			continue
		}

		// apply the copied impression object as an array
		// to the request object
		request.Imp = []openrtb.Imp{impCopy}

		// json marshal the request
		body, err := json.Marshal(request)
		if err != nil {
			errs = append(errs, err)
			continue
		}

		// assign the default uri
		uri := adapter.URI

		// assign adapter region based uri if it exists
		if endpoint, ok := adapter.SupportedRegions[Region(dv360Ext.Region)]; ok {
			uri = endpoint
		}

		// build request data object
		reqData := &adapters.RequestData{
			Method:  "POST",
			Uri:     uri,
			Body:    body,
			Headers: headers,

			TapjoyData: adapters.TapjoyData{
				Bidder: adapter.Name(),
				Region: dv360Ext.Region,
				SKAN: adapters.SKAN{
					Supported: dv360Ext.SKADNSupported,
					Sent:      skanSent,
				},
				MRAID: adapters.MRAID{
					Supported: dv360Ext.MRAIDSupported,
				},
			},
		}

		// append to request data array
		requestData = append(requestData, reqData)
	}

	return requestData, errs
}

func (adapter *DV360Adapter) MakeBids(_ *openrtb.BidRequest, externalRequest *adapters.RequestData, response *adapters.ResponseData) (*adapters.BidderResponse, []error) {
	if response.StatusCode == http.StatusNoContent {
		return nil, nil
	}

	if response.StatusCode == http.StatusBadRequest {
		return nil, []error{&errortypes.BadInput{
			Message: fmt.Sprintf("Unexpected status code: %d. Run with request.debug = 1 for more info", response.StatusCode),
		}}
	}

	if response.StatusCode != http.StatusOK {
		return nil, []error{&errortypes.BadServerResponse{
			Message: fmt.Sprintf("Unexpected status code: %d. Run with request.debug = 1 for more info", response.StatusCode),
		}}
	}

	var bidResp openrtb.BidResponse
	if err := json.Unmarshal(response.Body, &bidResp); err != nil {
		return nil, []error{&errortypes.BadServerResponse{
			Message: err.Error(),
		}}
	}

	if len(bidResp.SeatBid) == 0 {
		return nil, nil
	}

	bidResponse := adapters.NewBidderResponseWithBidsCapacity(len(bidResp.SeatBid[0].Bid))

	var bidReq openrtb.BidRequest
	if err := json.Unmarshal(externalRequest.Body, &bidReq); err != nil {
		return nil, []error{err}
	}

	bidType := openrtb_ext.BidTypeBanner

	if bidReq.Imp[0].Video != nil {
		bidType = openrtb_ext.BidTypeVideo
	}

	for _, sb := range bidResp.SeatBid {
		for _, b := range sb.Bid {
			if b.Price != 0 {
				bidResponse.Bids = append(bidResponse.Bids, &adapters.TypedBid{
					Bid:     &b,
					BidType: bidType,
				})
			}
		}
	}

	return bidResponse, nil
}