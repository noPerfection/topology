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

	return New("service_id", IndependentType), handlerOfType, handler2OfType, handlerOfType2
}

func TestServiceValidateTypes(t *testing.T) {
	_, handlerOfType, _, _ := testService()

	invalidHandler := Handler{Type: HandlerType("invalid_handler_type")}

	generatedService := &Service{
		Name:     "generated",
		Type:     "the_invalid_type",
		Handlers: []Handler{handlerOfType},
	}

	if err := generatedService.ValidateTypes(); err == nil {
		t.Fatal("ValidateTypes with invalid service type returned nil error")
	}

	generatedService.Type = IndependentType
	if err := generatedService.ValidateTypes(); err != nil {
		t.Fatalf("ValidateTypes valid service: %v", err)
	}

	generatedService.Handlers = []Handler{{Type: ReplierType}}
	if err := generatedService.ValidateTypes(); err == nil {
		t.Fatal("ValidateTypes with empty handler category returned nil error")
	}

	generatedService.Handlers = []Handler{invalidHandler}
	if err := generatedService.ValidateTypes(); err == nil {
		t.Fatal("ValidateTypes with invalid handler type returned nil error")
	}
}

func TestServiceValidateSocketBootstrap(t *testing.T) {
	service := Service{
		Type: ProxyType,
		Name: "inproc-service",
		Handlers: []Handler{
			{
				Type:     ReplierType,
				Category: "inproc",
				Endpoint: message.NewEndpoint("inproc-handler", 0),
			},
		},
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
		Handlers: []Handler{
			{
				Type:     ReplierType,
				Category: "tmp",
				Endpoint: message.NewEndpoint("tmp/service.sock", 0),
			},
		},
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
		Handlers: []Handler{
			{
				Type:     ReplierType,
				Category: "tcp",
				Endpoint: message.NewEndpoint("tcp-service", 4101),
			},
		},
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
		Proxies: []DepTarget{RefTarget("auth_proxy")},
	}); err != nil {
		t.Fatalf("ValidateDepService with proxies: %v", err)
	}

	if err := ValidateDepService(DepService{
		Name:       "get-user",
		Extensions: []DepTarget{RefTarget("user_service")},
	}); err != nil {
		t.Fatalf("ValidateDepService with extensions: %v", err)
	}
}

func TestServiceParametersNotValidated(t *testing.T) {
	serviceConfig, handlerOfType, _, _ := testService()
	serviceConfig.Handlers = []Handler{handlerOfType}
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
	serviceConfig.Handlers = []Handler{handlerOfType}
	serviceConfig.HandlerDeps = []DepService{{Name: "orphan"}}

	if err := ValidateService(*serviceConfig); err == nil {
		t.Fatal("ValidateService with invalid handler-deps returned nil error")
	}

	serviceConfig.HandlerDeps = []DepService{
		{
			Name: "public",
			Proxies: []DepTarget{
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
	serviceConfig.Handlers = []Handler{handlerOfType, handler2OfType, handlerOfType2}

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
	if foundHandler.Endpoint.Id != handlerOfType.Endpoint.Id {
		t.Fatalf("handler id = %q, want %q", foundHandler.Endpoint.Id, handlerOfType.Endpoint.Id)
	}
	if foundHandler.Category != handlerOfType.Category {
		t.Fatalf("handler category = %q, want %q", foundHandler.Category, handlerOfType.Category)
	}
}

func TestServiceGetHandler(t *testing.T) {
	serviceConfig, handlerOfType, _, handlerOfType2 := testService()
	serviceConfig.Handlers = []Handler{
		handlerOfType,
		{
			Type:     PairType,
			Category: "pair",
			Endpoint: message.NewEndpoint(handlerOfType.Endpoint.Id, 9999),
		},
		handlerOfType2,
	}

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
	if foundHandler.Type != handlerOfType.Type {
		t.Fatalf("handler type = %q, want %q", foundHandler.Type, handlerOfType.Type)
	}
}

func TestServiceSetHandler(t *testing.T) {
	serviceConfig, handlerOfType, _, handlerOfType2 := testService()

	if len(serviceConfig.Handlers) != 0 {
		t.Fatalf("initial len(Handlers) = %d, want 0", len(serviceConfig.Handlers))
	}

	var nilService *Service
	nilService.SetHandler(handlerOfType)

	serviceConfig.SetHandler(handlerOfType)
	if len(serviceConfig.Handlers) != 1 {
		t.Fatalf("len(Handlers) = %d, want 1", len(serviceConfig.Handlers))
	}
	if serviceConfig.Handlers[0].Type != ReplierType {
		t.Fatalf("handler type = %q, want %q", serviceConfig.Handlers[0].Type, ReplierType)
	}

	serviceConfig.SetHandler(handlerOfType2)
	if len(serviceConfig.Handlers) != 2 {
		t.Fatalf("len(Handlers) = %d, want 2", len(serviceConfig.Handlers))
	}
	if serviceConfig.Handlers[0].Type != ReplierType {
		t.Fatalf("first handler type = %q, want %q", serviceConfig.Handlers[0].Type, ReplierType)
	}
	if serviceConfig.Handlers[1].Type != SyncReplierType {
		t.Fatalf("second handler type = %q, want %q", serviceConfig.Handlers[1].Type, SyncReplierType)
	}

	updatedHandler := Handler{
		Type:     PairType,
		Category: "pair",
		Endpoint: message.NewEndpoint(handlerOfType.Endpoint.Id, 0),
	}
	serviceConfig.SetHandler(updatedHandler)
	if len(serviceConfig.Handlers) != 2 {
		t.Fatalf("len(Handlers) after update = %d, want 2", len(serviceConfig.Handlers))
	}
	if serviceConfig.Handlers[0].Type != PairType {
		t.Fatalf("first handler type = %q, want %q", serviceConfig.Handlers[0].Type, PairType)
	}
	if serviceConfig.Handlers[1].Type != SyncReplierType {
		t.Fatalf("second handler type = %q, want %q", serviceConfig.Handlers[1].Type, SyncReplierType)
	}
}

func TestServiceRemoveHandler(t *testing.T) {
	serviceConfig, handlerOfType, handler2OfType, handlerOfType2 := testService()
	serviceConfig.Handlers = []Handler{handlerOfType, handler2OfType, handlerOfType2}

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
	if serviceConfig.Handlers[0].Endpoint.Id != handlerOfType.Endpoint.Id {
		t.Fatalf("first handler id = %q, want %q", serviceConfig.Handlers[0].Endpoint.Id, handlerOfType.Endpoint.Id)
	}
	if serviceConfig.Handlers[1].Endpoint.Id != handlerOfType2.Endpoint.Id {
		t.Fatalf("second handler id = %q, want %q", serviceConfig.Handlers[1].Endpoint.Id, handlerOfType2.Endpoint.Id)
	}
}
