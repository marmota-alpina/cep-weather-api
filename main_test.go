package main

import (
	"encoding/json"
	"fmt"
	_ "io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// Mock HTTP server para simular ViaCEP e WeatherAPI
var mockServer *httptest.Server

// Variáveis globais para controlar as respostas do mock
var (
	mockViaCEPResponse       string
	mockViaCEPStatusCode     int
	mockWeatherAPIResponse   string
	mockWeatherAPIStatusCode int
	expectWeatherAPICity     string // Para verificar se a cidade correta está sendo passada
)

// mockHandler simula as APIs externas
func mockHandler(w http.ResponseWriter, r *http.Request) {
	if strings.Contains(r.URL.Path, "/ws/") { // ViaCEP request
		if mockViaCEPStatusCode == 0 {
			mockViaCEPStatusCode = http.StatusOK // Default
		}
		w.WriteHeader(mockViaCEPStatusCode)
		fmt.Fprintln(w, mockViaCEPResponse)
	} else if strings.Contains(r.URL.Path, "/v1/current.json") { // WeatherAPI request
		if mockWeatherAPIStatusCode == 0 {
			mockWeatherAPIStatusCode = http.StatusOK // Default
		}
		// Verifica se a cidade esperada está na query
		queryCity := r.URL.Query().Get("q")
		if expectWeatherAPICity != "" && queryCity != expectWeatherAPICity {
			w.WriteHeader(http.StatusBadRequest) // Simula erro se a cidade não for a esperada
			fmt.Fprintf(w, `{"error": {"code": 1006, "message": "Expected city %s but got %s"}}`, expectWeatherAPICity, queryCity)
			return
		}

		w.WriteHeader(mockWeatherAPIStatusCode)
		fmt.Fprintln(w, mockWeatherAPIResponse)
	} else {
		http.NotFound(w, r)
	}
}

func setup() {
	if mockServer == nil {
		mockServer = httptest.NewServer(http.HandlerFunc(mockHandler))
		httpClient = mockServer.Client()
		httpClient.Timeout = requestTimeout
		// Redireciona chamadas para o servidor mock durante os testes
		viaCEPURL = mockServer.URL
		weatherAPIURL = mockServer.URL
		weatherAPIKey = "be4bd84912cb4b25803234739252104"
	}

	// Reseta os mocks
	mockViaCEPResponse = ""
	mockViaCEPStatusCode = http.StatusOK
	mockWeatherAPIResponse = ""
	mockWeatherAPIStatusCode = http.StatusOK
	expectWeatherAPICity = ""
}

// teardown fecha o mock server após todos os testes
func teardown() {
	// Este será chamado implicitamente no final da execução dos testes
	// Não precisamos fechar explicitamente aqui com t.Cleanup()
	// Mas se quisessemos fechar após cada teste:
	// mockServer.Close()
}

func TestMain(m *testing.M) {
	// Configuração global antes de rodar os testes
	// Não precisamos chamar setup() aqui, pois ele será chamado dentro de cada TestXxx

	// Roda os testes
	exitCode := m.Run()

	// Limpeza global após todos os testes
	if mockServer != nil {
		mockServer.Close()
	}

	os.Exit(exitCode)
}

func TestWeatherHandler_Success(t *testing.T) {
	setup()          // Configura o mock para este teste
	defer teardown() // Garante limpeza se necessário

	cep := "01001000" // CEP da Praça da Sé, São Paulo
	expectedCity := "São Paulo"
	expectedTempC := 25.5

	// Configura as respostas do mock
	mockViaCEPResponse = fmt.Sprintf(`{"cep": "01001-000", "logradouro": "Praça da Sé", "complemento": "lado ímpar", "bairro": "Sé", "localidade": "%s", "uf": "SP", "ibge": "3550308", "gia": "1004", "ddd": "11", "siafi": "7107"}`, expectedCity)
	mockWeatherAPIResponse = fmt.Sprintf(`{"location": {"name": "%s"}, "current": {"temp_c": %.1f}}`, expectedCity, expectedTempC)
	expectWeatherAPICity = expectedCity // Garante que a cidade correta foi passada para WeatherAPI

	req := httptest.NewRequest(http.MethodGet, "/weather/"+cep, nil)
	rr := httptest.NewRecorder() // Recorder para capturar a resposta

	weatherHandler(rr, req) // Chama o handler

	// Verifica o status code
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
		t.Logf("Response body: %s", rr.Body.String()) // Loga o corpo em caso de erro
	}

	// Verifica o content type
	expectedContentType := "application/json"
	if ctype := rr.Header().Get("Content-Type"); ctype != expectedContentType {
		t.Errorf("handler returned wrong content type: got %s want %s", ctype, expectedContentType)
	}

	// Verifica o corpo da resposta
	var actualResponse WeatherResponse
	err := json.NewDecoder(rr.Body).Decode(&actualResponse)
	if err != nil {
		t.Fatalf("Could not decode response body: %v", err)
	}

	expectedResponse := WeatherResponse{
		TempC: expectedTempC,
		TempF: celsiusToFahrenheit(expectedTempC),
		TempK: celsiusToKelvin(expectedTempC),
	}

	if actualResponse != expectedResponse {
		t.Errorf("handler returned unexpected body: got %+v want %+v", actualResponse, expectedResponse)
	}
}

func TestWeatherHandler_InvalidCEPFormat(t *testing.T) {
	setup()
	defer teardown()

	invalidCeps := []string{"123", "123456789", "abcdefgh", "1234-567"}

	for _, cep := range invalidCeps {
		t.Run(cep, func(t *testing.T) { // Sub-teste para cada CEP inválido
			req := httptest.NewRequest(http.MethodGet, "/weather/"+cep, nil)
			rr := httptest.NewRecorder()

			weatherHandler(rr, req)

			if status := rr.Code; status != http.StatusUnprocessableEntity {
				t.Errorf("handler returned wrong status code for CEP %s: got %v want %v", cep, status, http.StatusUnprocessableEntity)
			}

			expectedBody := errorInvalidZipcode
			// Lê o corpo e remove o newline final adicionado por http.Error
			actualBody := strings.TrimSpace(rr.Body.String())
			if actualBody != expectedBody {
				t.Errorf("handler returned unexpected body for CEP %s: got '%s' want '%s'", cep, actualBody, expectedBody)
			}
		})
	}
}

func TestWeatherHandler_CEPNotFound_ViaCEP(t *testing.T) {
	setup()
	defer teardown()

	cep := "99999999" // CEP que não existe

	// Configura mock do ViaCEP para retornar erro
	mockViaCEPResponse = `{"erro": true}`
	mockViaCEPStatusCode = http.StatusOK // ViaCEP retorna 200 OK mesmo com erro no corpo

	req := httptest.NewRequest(http.MethodGet, "/weather/"+cep, nil)
	rr := httptest.NewRecorder()

	weatherHandler(rr, req)

	if status := rr.Code; status != http.StatusNotFound {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusNotFound)
	}

	expectedBody := errorCannotFindZip
	actualBody := strings.TrimSpace(rr.Body.String())
	if actualBody != expectedBody {
		t.Errorf("handler returned unexpected body: got '%s' want '%s'", actualBody, expectedBody)
	}
}

func TestWeatherHandler_CEPNotFound_WeatherAPI(t *testing.T) {
	setup()
	defer teardown()

	cep := "01001000" // CEP válido (São Paulo)
	cityFromViaCEP := "São Paulo"
	// Simular que a WeatherAPI não encontra essa cidade (embora vá encontrar na real)

	// Configura mock do ViaCEP para sucesso
	mockViaCEPResponse = fmt.Sprintf(`{"localidade": "%s"}`, cityFromViaCEP)

	// Configura mock da WeatherAPI para retornar erro de cidade não encontrada
	mockWeatherAPIResponse = `{"error": {"code": 1006, "message": "No matching location found."}}`
	mockWeatherAPIStatusCode = http.StatusBadRequest // Ou 400, como WeatherAPI costuma fazer
	expectWeatherAPICity = cityFromViaCEP            // Garante que a cidade correta foi pesquisada

	req := httptest.NewRequest(http.MethodGet, "/weather/"+cep, nil)
	rr := httptest.NewRecorder()

	weatherHandler(rr, req)

	if status := rr.Code; status != http.StatusNotFound {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusNotFound)
		t.Logf("Response body: %s", rr.Body.String())
	}

	expectedBody := errorCannotFindZip
	actualBody := strings.TrimSpace(rr.Body.String())
	if actualBody != expectedBody {
		t.Errorf("handler returned unexpected body: got '%s' want '%s'", actualBody, expectedBody)
	}
}

// Teste para simular um erro interno no ViaCEP (ex: timeout, 5xx)
func TestWeatherHandler_InternalError_ViaCEP(t *testing.T) {
	setup()
	defer teardown()

	cep := "01001000"

	// Configura mock do ViaCEP para retornar erro 500
	mockViaCEPResponse = `Internal Server Error` // Corpo não importa tanto aqui
	mockViaCEPStatusCode = http.StatusInternalServerError

	req := httptest.NewRequest(http.MethodGet, "/weather/"+cep, nil)
	rr := httptest.NewRecorder()

	weatherHandler(rr, req)

	if status := rr.Code; status != http.StatusInternalServerError {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusInternalServerError)
	}

	expectedBody := errorInternalServer
	actualBody := strings.TrimSpace(rr.Body.String())
	if actualBody != expectedBody {
		t.Errorf("handler returned unexpected body: got '%s' want '%s'", actualBody, expectedBody)
	}
}

// Teste para simular um erro interno na WeatherAPI (ex: timeout, 5xx, chave inválida)
func TestWeatherHandler_InternalError_WeatherAPI(t *testing.T) {
	setup()
	defer teardown()

	cep := "01001000"
	cityFromViaCEP := "São Paulo"

	// Configura mock do ViaCEP para sucesso
	mockViaCEPResponse = fmt.Sprintf(`{"localidade": "%s"}`, cityFromViaCEP)
	expectWeatherAPICity = cityFromViaCEP

	// Configura mock da WeatherAPI para retornar erro 500 (simulando falha interna)
	mockWeatherAPIResponse = `Weather API Service Unavailable`
	mockWeatherAPIStatusCode = http.StatusInternalServerError

	req := httptest.NewRequest(http.MethodGet, "/weather/"+cep, nil)
	rr := httptest.NewRecorder()

	weatherHandler(rr, req)

	if status := rr.Code; status != http.StatusInternalServerError {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusInternalServerError)
	}

	expectedBody := errorInternalServer
	actualBody := strings.TrimSpace(rr.Body.String())
	if actualBody != expectedBody {
		t.Errorf("handler returned unexpected body: got '%s' want '%s'", actualBody, expectedBody)
	}
}
