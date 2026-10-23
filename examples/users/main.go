// Command users serves a paginated user table and reusable user cards on
// the MVU runtime. GET /users?page=N dispatches ShowPage; HTMX requests get
// the results fragment, plain requests the full page.
package main

import (
	"crypto/rand"
	"log"
	"net/http"
	"strconv"

	"github.com/dmundt/go-mvu/middleware"
	"github.com/dmundt/go-mvu/session"
	"github.com/dmundt/go-mvu/static"
	"github.com/dmundt/go-mvu/store"
	"github.com/dmundt/go-mvu/theme"
	"github.com/dmundt/go-mvu/web"
)

func main() {
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		log.Fatal(err)
	}
	handler, err := newHandler(secret)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("users example listening on http://localhost:8081/users?page=1")
	log.Fatal(http.ListenAndServe("localhost:8081", handler))
}

func newHandler(secret []byte) (http.Handler, error) {
	sessions, err := session.New(secret, session.Options{Secure: false})
	if err != nil {
		return nil, err
	}
	csrf, err := middleware.NewCSRF(secret)
	if err != nil {
		return nil, err
	}
	preference, err := theme.NewPreference(secret, theme.PreferenceOptions{Secure: false})
	if err != nil {
		return nil, err
	}
	directory := syntheticUsers(42)
	runtime, err := web.NewRuntime(web.RuntimeConfig[string, UsersModel, UsersMessage]{
		App:    UsersApp{Directory: directory},
		Store:  store.NewMemory[string, UsersModel](),
		Keys:   web.KeyResolverFunc[string](sessionKey),
		View:   UsersView{},
		Theme:  theme.Default,
		Themes: preference,
	})
	if err != nil {
		return nil, err
	}

	mux := http.NewServeMux()
	mux.Handle("GET "+static.Prefix, static.Handler())
	mux.Handle("GET /users", runtime.Handle(web.Route[UsersMessage]{
		Method:  http.MethodGet,
		Pattern: "/users",
		Decode:  web.DecoderFunc[UsersMessage](decodeShowPage),
		Result:  web.Result{Mode: web.RenderAuto, Fragment: "results"},
	}))
	mux.HandleFunc("GET /users/cards", func(w http.ResponseWriter, r *http.Request) {
		request := web.NewRequestContext(r, theme.Default, preference)
		page := cardsPage(request, directory[:6])
		if err := web.WritePage(r.Context(), w, http.StatusOK, page); err != nil {
			log.Printf("render user cards: %v", err)
		}
	})
	mux.Handle("POST /theme", preference.Handler("/users"))

	var handler http.Handler = mux
	handler = csrf.Wrap(handler)
	handler = preference.Wrap(handler)
	handler = sessions.Wrap(handler)
	handler = middleware.RequestID(handler)
	handler = middleware.SecurityHeaders(handler)
	return middleware.Recover(handler), nil
}

// decodeShowPage reads ?page=N. Missing or invalid → page 1.
func decodeShowPage(r *http.Request) (UsersMessage, error) {
	number, err := strconv.Atoi(r.URL.Query().Get("page"))
	if err != nil {
		number = 1
	}
	return ShowPage{Page: number}, nil
}

func sessionKey(r *http.Request) (string, error) {
	id, ok := session.ID(r)
	if !ok {
		return "", http.ErrNoCookie
	}
	return id, nil
}
