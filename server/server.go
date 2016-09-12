package server

import (
	"encoding/json"
	"fmt"
	"github.com/julienschmidt/httprouter"
	"net/http"
)

type Context struct {
	W   http.ResponseWriter
	Req *http.Request
	RP  httprouter.Params
}

func (c *Context) Get(key string) string {
	c.Req.ParseForm()
	s := c.Req.Form.Get(key)
	if s == "" {
		s = c.RP.ByName(key)
	}
	return s
}

func (c *Context) Unmarshal(i *interface{}) error {
	return json.NewDecoder(c.Req.Body).Decode(i)
}

type Resource interface {
	Index(c *Context) (int, interface{})
	Show(c *Context) (int, interface{})
	Create(c *Context) (int, interface{})
	Update(c *Context) (int, interface{})
	Destroy(c *Context) (int, interface{})
}

type (
	IndexNotSupported   struct{}
	ShowNotSupported    struct{}
	CreateNotSupported  struct{}
	UpdateNotSupported  struct{}
	DestroyNotSupported struct{}
)

func (IndexNotSupported) Index(c *Context) (int, interface{}) {
	return 404, ""
}

func (ShowNotSupported) Show(c *Context) (int, interface{}) {
	return 404, ""
}

func (CreateNotSupported) Create(c *Context) (int, interface{}) {
	return 404, ""
}

func (UpdateNotSupported) Update(c *Context) (int, interface{}) {
	return 404, ""
}

func (DestroyNotSupported) Destroy(c *Context) (int, interface{}) {
	return 404, ""
}

func NewAPI() *API {
	return API{
		router: httprouter.New(),
	}
}

type API struct {
	router *httprouter.Router
}

func (api *API) Abort(rw http.ResponseWriter, statusCode int) {
	rw.WriteHeader(statusCode)
}

func (api *API) requestHandler(call func(c *Context) (int, interface{})) http.HandlerFunc {
	return func(rw http.ResponseWriter, request *http.Request, ps httprouter.Params) {

		ctx := &Context{
			W:   rw,
			Req: request,
			RP:  ps,
		}

		code, data := call(ctx)

		content, err := json.Marshal(data)
		if err != nil {
			api.Abort(rw, 500)
			return
		}

		rw.WriteHeader(code)
		rw.Write(content)
	}
}

func (api *API) AddResource(resource Resource, path string) {
	api.router.GET(path, api.requestHandler(resource.Index))
	api.router.GET(path+"/:id", api.requestHandler(resource.Show))
	api.router.POST(path, api.requestHandler(resource.Create))
	api.router.PATCH(path+"/:id", api.requestHandler(resource.Update))
	api.router.PUT(path+"/:id", api.requestHandler(resource.Update))
	api.router.DELETE(path+"/:id", api.requestHandler(resource.Destroy))
}

func (api *API) Start(port int) {
	portString := fmt.Sprintf(":%d", port)
	http.ListenAndServe(portString, nil)
}
