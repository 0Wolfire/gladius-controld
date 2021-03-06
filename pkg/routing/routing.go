package routing

import (
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/jinzhu/gorm"
	"github.com/spf13/viper"

	"github.com/gladiusio/gladius-controld/pkg/blockchain"
	"github.com/gladiusio/gladius-controld/pkg/p2p/peer"
	"github.com/gladiusio/gladius-controld/pkg/routing/handlers"
	ghandlers "github.com/gorilla/handlers"
	"github.com/gorilla/mux"
)

var apiRouter *mux.Router
var Database *gorm.DB

type ControlRouter struct {
	Router *mux.Router
	Port   string
	Debug  bool
}

func (cRouter *ControlRouter) Start() {
	if cRouter.Debug {
		cRouter.Router.Use(loggingMiddleware)
	}

	fmt.Println("Starting API at http://localhost:" + cRouter.Port)
	log.Fatal(http.ListenAndServe(":"+cRouter.Port, ghandlers.CORS()(cRouter.Router)))
}

func InitializeRouter() (*mux.Router, error) {
	router := mux.NewRouter()
	return router, nil
}

func InitializeAPISubRoutes(router *mux.Router) {
	// Base API Sub-Routes
	if apiRouter == nil {
		apiRouter = router.PathPrefix("/api").Subrouter()
		apiRouter.Use(responseMiddleware)
		apiRouter.NotFoundHandler = http.HandlerFunc(handlers.NotFoundHandler)
	}
}

func AppendP2PEndPoints(router *mux.Router, ga *blockchain.GladiusAccountManager) error {
	InitializeAPISubRoutes(router)

	// P2P setup
	peerStruct := peer.New(ga)
	p2pRouter := apiRouter.PathPrefix("/p2p").Subrouter()
	// P2P Message Routes
	p2pRouter.HandleFunc("/message/sign", handlers.CreateSignedMessageHandler(ga)).
		Methods(http.MethodPost)
	p2pRouter.HandleFunc("/message/verify", handlers.VerifySignedMessageHandler).
		Methods("POST")

	p2pRouter.HandleFunc("/network/join", handlers.JoinHandler(peerStruct)).
		Methods("POST")

	p2pRouter.HandleFunc("/network/leave", handlers.LeaveHandler(peerStruct)).
		Methods("POST")

	// P2P State Routes
	p2pRouter.HandleFunc("/state/push_message", handlers.PushStateMessageHandler(peerStruct)).
		Methods("POST")
	p2pRouter.HandleFunc("/state", handlers.GetFullStateHandler(peerStruct)).
		Methods("GET")
	p2pRouter.HandleFunc("/state/{node_address}", handlers.GetNodeStateHandler(peerStruct)).
		Methods("GET")
	p2pRouter.HandleFunc("/state/signatures", handlers.GetSignatureListHandler(peerStruct)).
		Methods("GET")
	p2pRouter.HandleFunc("/state/content_diff", handlers.GetContentNeededHandler(peerStruct)).
		Methods("POST")
	p2pRouter.HandleFunc("/state/content_links", handlers.GetContentLinksHandler(peerStruct)).
		Methods("POST")

	// Only enable for testing
	if viper.GetBool("NodeManager.Config.Debug") {
		p2pRouter.HandleFunc("/state/set_state", handlers.SetStateDebugHandler(peerStruct)).
			Methods("POST")
	}

	return nil
}

func AppendAccountManagementEndpoints(router *mux.Router) error {
	// Initialize Base API sub-route
	InitializeAPISubRoutes(router)

	// Account Management
	accountRouter := apiRouter.PathPrefix("/account/{address:0[xX][0-9a-fA-F]{40}}").Subrouter()
	accountRouter.HandleFunc("/balance/{symbol:[a-z]{3}}", handlers.AccountBalanceHandler)
	accountRouter.HandleFunc("/transactions", handlers.AccountTransactionsHandler).
		Methods(http.MethodPost)

	return nil
}

func AppendWalletManagementEndpoints(router *mux.Router, ga *blockchain.GladiusAccountManager) error {
	// Initialize Base API sub-route
	InitializeAPISubRoutes(router)

	// Key Management
	walletRouter := apiRouter.PathPrefix("/keystore").Subrouter()
	walletRouter.HandleFunc("/account/create", handlers.KeystoreAccountCreationHandler(ga)).
		Methods(http.MethodPost)
	walletRouter.HandleFunc("/account", handlers.KeystoreAccountRetrievalHandler(ga))
	walletRouter.HandleFunc("/account/open", handlers.KeystoreAccountUnlockHandler(ga)).
		Methods(http.MethodPost)

	return nil
}

func AppendStatusEndpoints(router *mux.Router) error {
	// Initialize Base API sub-route
	InitializeAPISubRoutes(router)

	// TxHash Status Sub-Routes
	statusRouter := apiRouter.PathPrefix("/status").Subrouter()
	statusRouter.HandleFunc("/", handlers.StatusHandler).
		Methods(http.MethodGet, http.MethodPut).
		Name("status")
	statusRouter.HandleFunc("/tx/{tx:0[xX][0-9a-fA-F]{64}}", handlers.StatusTxHandler).
		Methods(http.MethodGet).
		Name("status-tx")

	return nil
}

func AppendNodeManagerEndpoints(router *mux.Router, ga *blockchain.GladiusAccountManager) error {
	// Initialize Base API sub-route
	InitializeAPISubRoutes(router)

	// Node Sub-Routes
	nodeRouter := apiRouter.PathPrefix("/node").Subrouter()
	// Node pool applications
	nodeRouter.HandleFunc("/applications", handlers.NodeViewAllApplicationsHandler(ga)).
		Methods(http.MethodGet)
	// Node application to Pool
	nodeRouter.HandleFunc("/applications/{poolAddress:0[xX][0-9a-fA-F]{40}}/new", handlers.NodeNewApplicationHandler(ga)).
		Methods(http.MethodPost)
	nodeRouter.HandleFunc("/applications/{poolAddress:0[xX][0-9a-fA-F]{40}}/view", handlers.NodeViewApplicationHandler(ga)).
		Methods(http.MethodGet)

	// Pool Sub-Routes
	poolRouter := apiRouter.PathPrefix("/pool").Subrouter()
	// Retrieve owned Pool if available
	poolRouter.HandleFunc("/", nil)
	// Pool Retrieve Data
	poolRouter.HandleFunc("/{poolAddress:0[xX][0-9a-fA-F]{40}}", handlers.PoolPublicDataHandler(ga)).
		Methods(http.MethodGet)

	return nil
}

func AppendMarketEndpoints(router *mux.Router, ga *blockchain.GladiusAccountManager) error {
	// Initialize Base API sub-route
	InitializeAPISubRoutes(router)

	// Market Sub-Routes
	marketRouter := apiRouter.PathPrefix("/market").Subrouter()
	marketRouter.HandleFunc("/pools", handlers.MarketPoolsHandler(ga))

	return nil
}

func AppendPoolManagerEndpoints(router *mux.Router, ga *blockchain.GladiusAccountManager, db *gorm.DB) error {
	// Initialize Base API sub-route
	InitializeAPISubRoutes(router)

	// Pool
	poolRouter := apiRouter.PathPrefix("/pool").Subrouter()
	// Pool data, both public and private data can be set here
	poolRouter.HandleFunc("/{poolAddress:0[xX][0-9a-fA-F]{40}}/data", handlers.PoolPublicDataHandler(ga)).
		Methods(http.MethodGet)
	poolRouter.HandleFunc("/applications/pending/pool", handlers.PoolRetrievePendingPoolConfirmationApplicationsHandler(db)).
		Methods(http.MethodGet)
	poolRouter.HandleFunc("/applications/pending/node", handlers.PoolRetrievePendingNodeConfirmationApplicationsHandler(db)).
		Methods(http.MethodGet)
	poolRouter.HandleFunc("/applications/rejected", handlers.PoolRetrieveRejectedApplicationsHandler(db)).
		Methods(http.MethodGet)
	poolRouter.HandleFunc("/applications/approved", handlers.PoolRetrieveApprovedApplicationsHandler(db)).
		Methods(http.MethodGet)

	// Market
	marketRouter := apiRouter.PathPrefix("/market").Subrouter()
	marketRouter.HandleFunc("/pools/owned", handlers.MarketPoolsOwnedHandler(ga))
	marketRouter.HandleFunc("/pools/create", handlers.MarketPoolsCreateHandler(ga)).
		Methods(http.MethodPost)

	return nil
}

func AppendServerEndpoints(router *mux.Router, db *gorm.DB) error {
	// Initialize Base API sub-route
	InitializeAPISubRoutes(router)
	// Applications
	applicationRouter := apiRouter.PathPrefix("/server").Subrouter()
	applicationRouter.HandleFunc("/info", handlers.PublicPoolInformationHandler(db)).
		Methods(http.MethodGet)

	return nil
}

func AppendApplicationEndpoints(router *mux.Router, db *gorm.DB) error {
	// Initialize Base API sub-route
	InitializeAPISubRoutes(router)

	// Applications
	applicationRouter := apiRouter.PathPrefix("/applications").Subrouter()
	applicationRouter.HandleFunc("/new", handlers.PoolNewApplicationHandler(db)).
		Methods(http.MethodPost)
	applicationRouter.HandleFunc("/edit", handlers.PoolEditApplicationHandler(db)).
		Methods(http.MethodPost)
	applicationRouter.HandleFunc("/view", handlers.PoolViewApplicationHandler(db)).
		Methods(http.MethodPost)
	applicationRouter.HandleFunc("/status", handlers.PoolStatusViewHandler(db)).
		Methods(http.MethodPost)
	applicationRouter.HandleFunc("/pool/contains/{walletAddress:0[xX][0-9a-fA-F]{40}}", handlers.PoolContainsNode(db))
	applicationRouter.HandleFunc("/nodes", handlers.PoolNodes(db))

	return nil
}

func responseMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "application/json")
		if next != nil {
			next.ServeHTTP(w, r)
		}
	})
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Println()
		log.Println(formatRequest(r))
		log.Println()

		next.ServeHTTP(w, r)
	})
}

func formatRequest(r *http.Request) string {
	// Create return string
	var request []string

	// Add the request string
	url := fmt.Sprintf("%v %v %v", r.Method, r.URL, r.Proto)
	request = append(request, url)

	// Add the host
	request = append(request, fmt.Sprintf("Host: %v", r.Host))

	// Loop through headers
	for name, headers := range r.Header {
		name = strings.ToLower(name)
		for _, h := range headers {
			request = append(request, fmt.Sprintf("%v: %v", name, h))
		}
	}

	// If this is a POST, add post data
	if r.Method == http.MethodPost {
		r.ParseForm()
		request = append(request, "\n")
		request = append(request, r.Form.Encode())
	}

	// Return the request as a string
	return strings.Join(request, "\n")
}
