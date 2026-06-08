package config

import (
	"encoding/json"
	"testing"

	"github.com/noPerfection/datatype"
	"github.com/noPerfection/protocol/message"
)

func testService() (*Service, Handler, Handler, Handler) {
	handlerOfType := Handler{
		Type:     ReplierType,
		Category: "public",
		Endpoint: message.NewEndpoint("handler_1", 4101),
	}
	handler2OfType := Handler{
		Type:     ReplierType,
		Category: "internal",
		Endpoint: message.NewEndpoint("handler_2", 4102),
	}
	handlerOfType2 := Handler{
		Type:     SyncReplierType,
		Category: "sync",
		Endpoint: message.NewEndpoint("handler_3", 4103),
	}

	return &Service{
		Type:     IndependentType,
		Name:     "service_id",
		Handlers: make([]HandlerVariant, 0),
	}, handlerOfType, handler2OfType, handlerOfType2
}

func TestValidateService(t *testing.T) {
	_, handlerOfType, _, _ := testService()

	invalidHandler := Handler{Type: HandlerType("invalid_handler_type")}

	generatedService := &Service{
		Name:     "generated",
		Type:     "the_invalid_type",
		Handlers: NewHandlerVariants(handlerOfType),
	}

	if err := ValidateService(*generatedService); err == nil {
		t.Fatal("ValidateService with invalid service type returned nil error")
	}

	generatedService.Type = IndependentType
	if err := ValidateService(*generatedService); err != nil {
		t.Fatalf("ValidateService valid service: %v", err)
	}

	generatedService.Handlers = NewHandlerVariants(Handler{Type: ReplierType})
	if err := ValidateService(*generatedService); err == nil {
		t.Fatal("ValidateService with empty handler category returned nil error")
	}

	generatedService.Handlers = NewHandlerVariants(invalidHandler)
	if err := ValidateService(*generatedService); err == nil {
		t.Fatal("ValidateService with invalid handler type returned nil error")
	}
}

func TestServiceValidateSocketBootstrap(t *testing.T) {
	service := Service{
		Type: ProxyType,
		Name: "inproc-service",
		Handlers: NewHandlerVariants(
			Handler{
				Type:     ReplierType,
				Category: "inproc",
				Endpoint: message.NewEndpoint("inproc-handler", 0),
			},
		),
	}
	if err := ValidateService(service); err == nil {
		t.Fatal("ValidateService with inproc endpoint and no module-url returned nil error")
	}

	service.ModuleUrl = "github.com/noPerfection/inproc-service"
	if err := ValidateService(service); err != nil {
		t.Fatalf("ValidateService with module-url: %v", err)
	}

	service = Service{
		Type: ProxyType,
		Name: "tmp-service",
		Handlers: NewHandlerVariants(
			Handler{
				Type:     ReplierType,
				Category: "tmp",
				Endpoint: message.NewEndpoint("tmp/service.sock", 0),
			},
		),
	}
	if err := ValidateService(service); err == nil {
		t.Fatal("ValidateService with ipc endpoint and no start-command returned nil error")
	}

	service.StartCommand = "go run ./cmd/tmp-service"
	if err := ValidateService(service); err != nil {
		t.Fatalf("ValidateService with start-command: %v", err)
	}

	service = Service{
		Type: ProxyType,
		Name: "tcp-service",
		Handlers: NewHandlerVariants(
			Handler{
				Type:     ReplierType,
				Category: "tcp",
				Endpoint: message.NewEndpoint("tcp-service", 4101),
			},
		),
	}
	if err := ValidateService(service); err != nil {
		t.Fatalf("ValidateService with tcp endpoint and no bootstrap fields: %v", err)
	}
}

func TestValidateDepService(t *testing.T) {
	if err := ValidateDepService(DepService{Name: "orphan"}); err == nil {
		t.Fatal("ValidateDepService without proxies or extensions returned nil error")
	}

	if err := ValidateDepService(DepService{
		Name:    "call-user-api",
		Proxies: []ServicePointer{RefTarget("auth_proxy")},
	}); err != nil {
		t.Fatalf("ValidateDepService with proxies: %v", err)
	}

	if err := ValidateDepService(DepService{
		Name:       "get-user",
		Extensions: []ServicePointer{RefTarget("user_service")},
	}); err != nil {
		t.Fatalf("ValidateDepService with extensions: %v", err)
	}
}

func TestValidateProxyForwards(t *testing.T) {
	proxyHandler := ProxyHandler{
		Handler: Handler{
			Type:     SyncReplierType,
			Category: "main",
			Endpoint: message.NewEndpoint("proxy", 4101),
		},
		Routes: []string{"hello", "age-verification"},
		Outbounds: []ServicePointer{
			RefTarget("default-name-proxy", "main"),
			ServiceTarget(Service{
				Type:      IndependentType,
				Name:      "hello-world",
				ModuleUrl: "github.com/noPerfection/hello-world",
				Handlers: NewHandlerVariants(Handler{
					Type:     ReplierType,
					Category: "main",
					Endpoint: message.NewEndpoint("hello-world", 4102),
				}),
			}),
		},
		Forward: []map[string]string{
			{
				"hello":            "default-name-proxy/main",
				"age-verification": "hello-world",
			},
		},
	}
	service := Service{
		Type:      ProxyType,
		Name:      "proxy",
		ModuleUrl: "github.com/noPerfection/proxy",
		Handlers:  []HandlerVariant{NewProxyHandlerVariant(proxyHandler)},
	}
	if err := ValidateService(service); err != nil {
		t.Fatalf("ValidateService with forward mappings: %v", err)
	}

	proxyHandler.Forward = []map[string]string{{"missing-route": "hello-world"}}
	service.Handlers = []HandlerVariant{NewProxyHandlerVariant(proxyHandler)}
	if err := ValidateService(service); err == nil {
		t.Fatal("ValidateService with forward route missing from routes returned nil error")
	}

	proxyHandler.Forward = []map[string]string{{"hello": "missing-service"}}
	service.Handlers = []HandlerVariant{NewProxyHandlerVariant(proxyHandler)}
	if err := ValidateService(service); err == nil {
		t.Fatal("ValidateService with forward outbound missing from outbounds returned nil error")
	}
}

func TestProxyHandlerUnmarshalForwardOnly(t *testing.T) {
	data := []byte(`{
		"type": "SyncReplier",
		"category": "main",
		"endpoint": {"id": "proxy", "port": 4101},
		"forward": [{"hello": "hello-world"}],
		"outbounds": ["hello-world"]
	}`)

	var variant HandlerVariant
	if err := json.Unmarshal(data, &variant); err != nil {
		t.Fatalf("json.Unmarshal proxy handler with forward: %v", err)
	}
	if variant.ProxyHandler == nil {
		t.Fatal("variant.ProxyHandler is nil")
	}
	if len(variant.ProxyHandler.Forward) != 1 || variant.ProxyHandler.Forward[0]["hello"] != "hello-world" {
		t.Fatalf("Forward = %#v, want hello mapping", variant.ProxyHandler.Forward)
	}
}

func TestServiceParametersNotValidated(t *testing.T) {
	serviceConfig, handlerOfType, _, _ := testService()
	serviceConfig.Handlers = NewHandlerVariants(handlerOfType)
	serviceConfig.Parameters = datatype.New().Set("region", "eu-west")

	if err := ValidateService(*serviceConfig); err != nil {
		t.Fatalf("ValidateService with parameters: %v", err)
	}
}

func TestServiceParametersJSONRoundTrip(t *testing.T) {
	data := []byte(`{
		"type": "Proxy",
		"name": "worker",
		"parameters": {
			"region": "eu-west",
			"replicas": 3
		},
		"handlers": [{
			"type": "Replier",
			"category": "manager",
			"endpoint": {"id": "worker_1", "port": 6001}
		}]
	}`)

	var service Service
	if err := json.Unmarshal(data, &service); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}

	region, err := service.Parameters.StringValue("region")
	if err != nil {
		t.Fatalf("Parameters.StringValue('region'): %v", err)
	}
	if region != "eu-west" {
		t.Fatalf("region = %q, want eu-west", region)
	}

	replicas, err := service.Parameters.Uint64Value("replicas")
	if err != nil {
		t.Fatalf("Parameters.Uint64Value('replicas'): %v", err)
	}
	if replicas != 3 {
		t.Fatalf("replicas = %d, want 3", replicas)
	}

	out, err := json.Marshal(service)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}

	var roundTrip Service
	if err := json.Unmarshal(out, &roundTrip); err != nil {
		t.Fatalf("json.Unmarshal round trip: %v", err)
	}
	if roundTrip.Parameters == nil {
		t.Fatal("Parameters is nil after round trip")
	}
	if got, err := roundTrip.Parameters.StringValue("region"); err != nil || got != "eu-west" {
		t.Fatalf("round trip region = %q, err = %v", got, err)
	}
}

func TestServiceValidateHandlerDeps(t *testing.T) {
	serviceConfig, handlerOfType, _, _ := testService()
	serviceConfig.Handlers = NewHandlerVariants(handlerOfType)
	serviceConfig.HandlerDeps = []DepService{{Name: "orphan"}}

	if err := ValidateService(*serviceConfig); err == nil {
		t.Fatal("ValidateService with invalid handler-deps returned nil error")
	}

	serviceConfig.HandlerDeps = []DepService{
		{
			Name: "public",
			Proxies: []ServicePointer{
				RefTarget("auth_proxy"),
			},
		},
	}
	if err := ValidateService(*serviceConfig); err != nil {
		t.Fatalf("ValidateService with handler-deps: %v", err)
	}
}

func TestServiceHandlerByCategory(t *testing.T) {
	serviceConfig, handlerOfType, handler2OfType, handlerOfType2 := testService()
	serviceConfig.Handlers = NewHandlerVariants(handlerOfType, handler2OfType, handlerOfType2)

	if _, err := serviceConfig.HandlerByCategory(""); err == nil {
		t.Fatal("HandlerByCategory with empty category returned nil error")
	}
	if _, err := serviceConfig.HandlerByCategory("missing"); err == nil {
		t.Fatal("HandlerByCategory with missing category returned nil error")
	}

	foundHandler, err := serviceConfig.HandlerByCategory("public")
	if err != nil {
		t.Fatalf("HandlerByCategory public: %v", err)
	}
	handler := foundHandler.AsHandler()
	if handler.Endpoint.Id != handlerOfType.Endpoint.Id {
		t.Fatalf("handler id = %q, want %q", handler.Endpoint.Id, handlerOfType.Endpoint.Id)
	}
	if handler.Category != handlerOfType.Category {
		t.Fatalf("handler category = %q, want %q", handler.Category, handlerOfType.Category)
	}
}

func TestServiceGetHandler(t *testing.T) {
	serviceConfig, handlerOfType, _, handlerOfType2 := testService()
	serviceConfig.Handlers = NewHandlerVariants(
		handlerOfType,
		Handler{
			Type:     PairType,
			Category: "pair",
			Endpoint: message.NewEndpoint(handlerOfType.Endpoint.Id, 9999),
		},
		handlerOfType2,
	)

	if _, err := serviceConfig.GetHandler(message.Endpoint{}); err == nil {
		t.Fatal("GetHandler with empty id returned nil error")
	}
	if _, err := serviceConfig.GetHandler(message.NewEndpoint(handlerOfType.Endpoint.Id, 1234)); err == nil {
		t.Fatal("GetHandler with missing endpoint returned nil error")
	}

	foundHandler, err := serviceConfig.GetHandler(handlerOfType.Endpoint)
	if err != nil {
		t.Fatalf("GetHandler: %v", err)
	}
	handler := foundHandler.AsHandler()
	if handler.Type != handlerOfType.Type {
		t.Fatalf("handler type = %q, want %q", handler.Type, handlerOfType.Type)
	}
}

func TestServiceSetHandler(t *testing.T) {
	serviceConfig, handlerOfType, _, handlerOfType2 := testService()

	if len(serviceConfig.Handlers) != 0 {
		t.Fatalf("initial len(Handlers) = %d, want 0", len(serviceConfig.Handlers))
	}

	var nilService *Service
	nilService.SetHandler(NewHandlerVariant(handlerOfType))

	serviceConfig.SetHandler(NewHandlerVariant(handlerOfType))
	if len(serviceConfig.Handlers) != 1 {
		t.Fatalf("len(Handlers) = %d, want 1", len(serviceConfig.Handlers))
	}
	if serviceConfig.Handlers[0].AsHandler().Type != ReplierType {
		t.Fatalf("handler type = %q, want %q", serviceConfig.Handlers[0].AsHandler().Type, ReplierType)
	}

	serviceConfig.SetHandler(NewHandlerVariant(handlerOfType2))
	if len(serviceConfig.Handlers) != 2 {
		t.Fatalf("len(Handlers) = %d, want 2", len(serviceConfig.Handlers))
	}
	if serviceConfig.Handlers[0].AsHandler().Type != ReplierType {
		t.Fatalf("first handler type = %q, want %q", serviceConfig.Handlers[0].AsHandler().Type, ReplierType)
	}
	if serviceConfig.Handlers[1].AsHandler().Type != SyncReplierType {
		t.Fatalf("second handler type = %q, want %q", serviceConfig.Handlers[1].AsHandler().Type, SyncReplierType)
	}

	updatedHandler := Handler{
		Type:     PairType,
		Category: "pair",
		Endpoint: handlerOfType.Endpoint,
	}
	serviceConfig.SetHandler(NewHandlerVariant(updatedHandler))
	if len(serviceConfig.Handlers) != 2 {
		t.Fatalf("len(Handlers) after update = %d, want 2", len(serviceConfig.Handlers))
	}
	if serviceConfig.Handlers[0].AsHandler().Type != PairType {
		t.Fatalf("first handler type = %q, want %q", serviceConfig.Handlers[0].AsHandler().Type, PairType)
	}
	if serviceConfig.Handlers[1].AsHandler().Type != SyncReplierType {
		t.Fatalf("second handler type = %q, want %q", serviceConfig.Handlers[1].AsHandler().Type, SyncReplierType)
	}
}

func TestServiceRemoveHandler(t *testing.T) {
	serviceConfig, handlerOfType, handler2OfType, handlerOfType2 := testService()
	serviceConfig.Handlers = NewHandlerVariants(handlerOfType, handler2OfType, handlerOfType2)

	if err := serviceConfig.RemoveHandler(message.Endpoint{}); err == nil {
		t.Fatal("RemoveHandler with empty endpoint returned nil error")
	}
	if err := serviceConfig.RemoveHandler(message.NewEndpoint(handlerOfType.Endpoint.Id, 9999)); err == nil {
		t.Fatal("RemoveHandler with missing endpoint returned nil error")
	}

	if err := serviceConfig.RemoveHandler(handler2OfType.Endpoint); err != nil {
		t.Fatalf("RemoveHandler: %v", err)
	}
	if len(serviceConfig.Handlers) != 2 {
		t.Fatalf("len(Handlers) = %d, want 2", len(serviceConfig.Handlers))
	}
	if serviceConfig.Handlers[0].AsHandler().Endpoint.Id != handlerOfType.Endpoint.Id {
		t.Fatalf("first handler id = %q, want %q", serviceConfig.Handlers[0].AsHandler().Endpoint.Id, handlerOfType.Endpoint.Id)
	}
	if serviceConfig.Handlers[1].AsHandler().Endpoint.Id != handlerOfType2.Endpoint.Id {
		t.Fatalf("second handler id = %q, want %q", serviceConfig.Handlers[1].AsHandler().Endpoint.Id, handlerOfType2.Endpoint.Id)
	}
}
