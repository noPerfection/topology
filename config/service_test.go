package config

import (
	"testing"

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

func TestValidateCommandDep(t *testing.T) {
	if err := ValidateCommandDep(CommandDep{Command: "orphan"}); err == nil {
		t.Fatal("ValidateCommandDep without proxies or extensions returned nil error")
	}

	if err := ValidateCommandDep(CommandDep{
		Command: "call-user-api",
		Proxies: []DepTarget{RefTarget("auth_proxy")},
	}); err != nil {
		t.Fatalf("ValidateCommandDep with proxies: %v", err)
	}

	if err := ValidateCommandDep(CommandDep{
		Command:    "get-user",
		Extensions: []DepTarget{RefTarget("user_service")},
	}); err != nil {
		t.Fatalf("ValidateCommandDep with extensions: %v", err)
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
