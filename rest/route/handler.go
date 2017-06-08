package route

import (
	"net/http"

	"github.com/evergreen-ci/evergreen/rest"
	"github.com/evergreen-ci/evergreen/rest/data"
	"github.com/evergreen-ci/evergreen/rest/model"
	"github.com/evergreen-ci/evergreen/util"
	"github.com/mongodb/grip"
	"golang.org/x/net/context"
)

// MethodHandler contains all of the methods necessary for completely processing
// an API request. It contains an Authenticator to control access to the method
// and a RequestHandler to perform the required work for the request.
type MethodHandler struct {
	// PrefetchFunctions is a list of functions to be run before the main request
	// is executed.
	PrefetchFunctions []PrefetchFunc
	// MethodType is the HTTP Method Type that this handler will handler.
	// POST, PUT, DELETE, etc.
	MethodType string

	Authenticator
	RequestHandler
}

// ResponseData holds the information that the handler function will need to form
// its encoded response. A ResponseData is generated by a RequestHandler's Execute
// function and parsed in the main handler method.
type ResponseData struct {
	// Result is the resulting API models that the API request needs to return
	// to the user, either because they were queried for or because they were
	// created by this request.
	Result []model.Model

	// Metadata is an interface that holds any additional data that the handler
	// will need for encoding the API response.
	Metadata interface{}
}

// RequestHandler is an interface that defines how to process an HTTP request
// against an API resource.
type RequestHandler interface {
	// Handler defines how to fetch a new version of this handler.
	Handler() RequestHandler

	// ParseAndValidate defines how to retrieve the needed parameters from the HTTP
	// request. All needed data should be retrieved during the parse function since
	// other functions do not have access to the HTTP request.
	ParseAndValidate(context.Context, *http.Request) error

	// Execute performs the necessary work on the evergreen backend and returns
	// an API model to be surfaced to the user.
	Execute(context.Context, data.Connector) (ResponseData, error)
}

// makeHandler makes an http.HandlerFunc that wraps calls to each of the api
// Method functions. It marshalls the response to JSON and writes it out to
// as the response. If any of the functions return an error, it handles creating
// a JSON error and sending it as the response.
func makeHandler(methodHandler MethodHandler, sc data.Connector) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var err error
		ctx := context.Background()

		for _, pf := range methodHandler.PrefetchFunctions {
			if ctx, err = pf(ctx, sc, r); err != nil {
				handleAPIError(err, w, r)
				return
			}
		}

		if err = methodHandler.Authenticate(ctx, sc); err != nil {
			handleAPIError(err, w, r)
			return
		}
		reqHandler := methodHandler.RequestHandler.Handler()

		if err = reqHandler.ParseAndValidate(ctx, r); err != nil {
			handleAPIError(err, w, r)
			return
		}
		result, err := reqHandler.Execute(ctx, sc)
		if err != nil {
			handleAPIError(err, w, r)
			return
		}

		// Check the type of the results metadata. If it is a PaginationMetadata,
		// create the pagination headers. Otherwise, no additional processing is needed.
		// NOTE: This could expand to include additional metadata types that define
		// other specific cases for how to handle results.
		switch m := result.Metadata.(type) {
		case *PaginationMetadata:
			err := m.MakeHeader(w, sc.GetURL(), r.URL.Path)
			if err != nil {
				handleAPIError(err, w, r)
				return
			}
			util.WriteJSON(&w, result.Result, http.StatusOK)
		default:
			if len(result.Result) < 1 {
				http.Error(w, "{}", http.StatusInternalServerError)
				return
			} else if len(result.Result) > 1 {
				util.WriteJSON(&w, result.Result, http.StatusOK)
			} else {
				util.WriteJSON(&w, result.Result[0], http.StatusOK)
			}
		}
	}
}

// handleAPIError handles writing the given error to the response writer.
// It checks if the given error is an APIError and turns it into JSON to be
// written back to the requester. If the error is unknown, it must have come
// from a service layer package, in which case it is an internal server error
// and is returned as such.
func handleAPIError(e error, w http.ResponseWriter, r *http.Request) {
	apiErr := rest.APIError{}

	apiErr.StatusCode = http.StatusInternalServerError
	apiErr.Message = e.Error()

	if castError, ok := e.(rest.APIError); ok {
		apiErr = castError
		grip.Warningln("User error", r.Method, r.URL, e)
	} else {
		grip.Errorf("Service error %s %s %+v", r.Method, r.URL, e)
	}

	util.WriteJSON(&w, apiErr, apiErr.StatusCode)
}
