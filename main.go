package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"
)

// Variáveis globais para clientes HTTP e chave da API
var (
	httpClient    *http.Client
	weatherAPIKey string
	viaCEPURL     string = "https://viacep.com.br"
	weatherAPIURL string = "https://api.weatherapi.com"
)

// ViaCEPResponse Struct para a resposta da API ViaCEP
type ViaCEPResponse struct {
	Localidade string `json:"localidade"` // Cidade
	Erro       bool   `json:"erro"`
}

// WeatherAPIResponse Struct para a resposta da API WeatherAPI (parte relevante)
type WeatherAPIResponse struct {
	Current struct {
		TempC float64 `json:"temp_c"`
	} `json:"current"`
	Error *WeatherAPIError `json:"error,omitempty"` // Ponteiro para detectar ausência de erro
}

// WeatherAPIError Struct para erros da WeatherAPI
type WeatherAPIError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// WeatherResponse Struct para a resposta final da nossa API
type WeatherResponse struct {
	TempC float64 `json:"temp_C"`
	TempF float64 `json:"temp_F"`
	TempK float64 `json:"temp_K"`
}

const (
	viaCEPURLFormat        = "%s/ws/%s/json/"
	weatherAPIURLFormat    = "%s/v1/current.json?key=%s&q=%s&aqi=no"
	requestTimeout         = 10 * time.Second
	defaultPort            = "8080"
	weatherAPIEnvVar       = "WEATHER_API_KEY"
	errorInvalidZipcode    = "invalid zipcode"
	errorCannotFindZip     = "can not find zipcode"
	errorInternalServer    = "internal server error"
	errorMissingAPIKey     = "WeatherAPI key not configured"
	weatherAPINotFoundCode = 1006 // Código específico da WeatherAPI para "No matching location found."
)

// Regex para validar o formato do CEP (8 dígitos numéricos)
var cepRegex = regexp.MustCompile(`^\d{8}$`)

func main() {
	// Inicializa o cliente HTTP
	httpClient = &http.Client{
		Timeout: requestTimeout,
	}

	// Pega a chave da API do WeatherAPI das variáveis de ambiente
	weatherAPIKey = os.Getenv(weatherAPIEnvVar)
	if weatherAPIKey == "" {
		log.Fatalf("%s environment variable not set", weatherAPIEnvVar)
	}

	// Define o handler da rota principal
	http.HandleFunc("/weather/", weatherHandler) // Usar /weather/ para capturar o CEP na URL

	// Define a porta que a aplicação vai escutar
	port := os.Getenv("PORT")
	if port == "" {
		port = defaultPort
	}

	log.Printf("Server starting on port %s\n", port)
	// Inicia o servidor HTTP
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

// weatherHandler é o handler principal para a rota /weather/{cep}
func weatherHandler(w http.ResponseWriter, r *http.Request) {
	// Extrai o CEP da URL path
	// Ex: /weather/12345678 -> parts = ["", "weather", "12345678"]
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/"), "/")
	if len(parts) != 2 || parts[0] != "weather" {
		http.Error(w, "Usage: /weather/{cep}", http.StatusNotFound) // Ou Bad Request
		return
	}
	cep := parts[1]

	// 1. Valida o formato do CEP
	if !isValidCEP(cep) {
		http.Error(w, errorInvalidZipcode, http.StatusUnprocessableEntity) // 422
		return
	}

	// 2. Busca a cidade usando o ViaCEP
	cityName, err := getCityFromCEP(r.Context(), cep)
	if err != nil {
		// Verifica se o erro é "não encontrado" ou outro erro
		if err.Error() == errorCannotFindZip {
			http.Error(w, errorCannotFindZip, http.StatusNotFound) // 404
		} else {
			log.Printf("Error getting city from CEP %s: %v", cep, err)
			http.Error(w, errorInternalServer, http.StatusInternalServerError) // 500
		}
		return
	}

	// 3. Busca a temperatura usando a WeatherAPI
	tempC, err := getWeatherForCity(r.Context(), cityName)
	if err != nil {
		// Verifica se o erro é "não encontrado" ou outro erro
		if err.Error() == errorCannotFindZip {
			// Mapeia o erro de cidade não encontrada na WeatherAPI para o erro 404 do requisito
			http.Error(w, errorCannotFindZip, http.StatusNotFound) // 404
		} else {
			log.Printf("Error getting weather for city %s (from CEP %s): %v", cityName, cep, err)
			http.Error(w, errorInternalServer, http.StatusInternalServerError) // 500
		}
		return
	}

	// 4. Calcula as temperaturas em F e K
	tempF := celsiusToFahrenheit(tempC)
	tempK := celsiusToKelvin(tempC)

	// 5. Prepara a resposta de sucesso
	response := WeatherResponse{
		TempC: tempC,
		TempF: tempF,
		TempK: tempK,
	}

	// 6. Envia a resposta JSON
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK) // 200
	if err := json.NewEncoder(w).Encode(response); err != nil {
		// Loga o erro, mas não tenta escrever mais na resposta, pois o header já foi enviado
		log.Printf("Error encoding success response for CEP %s: %v", cep, err)
	}
}

// isValidCEP verifica se a ‘string’ do CEP tem 8 dígitos numéricos
func isValidCEP(cep string) bool {
	return cepRegex.MatchString(cep)
}

// getCityFromCEP busca a cidade correspondente a um CEP usando a API ViaCEP
func getCityFromCEP(ctx context.Context, cep string) (string, error) {
	cepURL := fmt.Sprintf(viaCEPURLFormat, viaCEPURL, cep)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, cepURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create ViaCEP request: %w", err)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to execute ViaCEP request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ViaCEP request failed with status: %s", resp.Status)
	}

	var viaCEPResp ViaCEPResponse
	if err := json.NewDecoder(resp.Body).Decode(&viaCEPResp); err != nil {
		return "", fmt.Errorf(errorCannotFindZip)
	}

	// ViaCEP retorna {"erro": true} para CEPs não encontrados
	if viaCEPResp.Erro || viaCEPResp.Localidade == "" {
		return "", fmt.Errorf(errorCannotFindZip)
	}

	log.Printf("CEP %s resolved to city: %s", cep, viaCEPResp.Localidade)
	return viaCEPResp.Localidade, nil
}

// getWeatherForCity busca a temperatura atual (Celsius) para uma cidade usando a WeatherAPI
func getWeatherForCity(ctx context.Context, cityName string) (float64, error) {
	// Codifica o nome da cidade para ser seguro na URL
	encodedCityName := url.QueryEscape(cityName)
	weatherRequestURL := fmt.Sprintf(weatherAPIURLFormat, weatherAPIURL, weatherAPIKey, encodedCityName)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, weatherRequestURL, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to create WeatherAPI request: %w", err)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("failed to execute WeatherAPI request: %w", err)
	}
	defer resp.Body.Close()

	// WeatherAPI retorna erros no corpo JSON, mesmo com status 200 OK às vezes,
	// mas também usa códigos de status HTTP para erros (ex: 400, 401, 403).
	// Precisamos decodificar a resposta para verificar ambos.

	var weatherResp WeatherAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&weatherResp); err != nil {
		// Se falhar a decodificação, verifica o status code HTTP
		if resp.StatusCode != http.StatusOK {
			return 0, fmt.Errorf("WeatherAPI request failed with status %s and couldn't decode error body", resp.Status)
		}
		// Se o status for OK, mas não decodificou, é um erro inesperado no formato da resposta
		return 0, fmt.Errorf("failed to decode WeatherAPI response even with status OK: %w", err)
	}

	// Verifica se há um erro na estrutura da resposta JSON
	if weatherResp.Error != nil {
		// Verifica se o erro é específico de cidade não encontrada
		if weatherResp.Error.Code == weatherAPINotFoundCode {
			log.Printf("WeatherAPI could not find city '%s'. Error code: %d, Message: %s", cityName, weatherResp.Error.Code, weatherResp.Error.Message)
			return 0, fmt.Errorf(errorCannotFindZip) // Mapeia para o erro 404 da nossa API
		}
		// Outro erro da WeatherAPI
		return 0, fmt.Errorf("WeatherAPI error: code %d, message: %s", weatherResp.Error.Code, weatherResp.Error.Message)
	}

	// Verifica o status HTTP também, como uma camada extra
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("WeatherAPI request failed with status: %s (but no error structure in body)", resp.Status)
	}

	log.Printf("Weather for city %s: %.1f°C", cityName, weatherResp.Current.TempC)
	return weatherResp.Current.TempC, nil
}

// celsiusToFahrenheit converte Celsius para Fahrenheit
func celsiusToFahrenheit(celsius float64) float64 {
	// F = C * 1.8 + 32
	fahrenheit := celsius*1.8 + 32
	// Arredondar para 1 casa decimal, se necessário (opcional, mas bom para consistência)
	return roundFloat(fahrenheit, 1)
}

// celsiusToKelvin converte Celsius para Kelvin
func celsiusToKelvin(celsius float64) float64 {
	// K = C + 273 (conforme especificado, embora 273.15 seja mais preciso)
	kelvin := celsius + 273
	return roundFloat(kelvin, 1) // Arredondar também
}

// roundFloat arredonda um float para um número específico de casas decimais
func roundFloat(val float64, precision uint) float64 {
	ratio := float64(1)
	for i := uint(0); i < precision; i++ {
		ratio *= 10
	}
	return float64(int(val*ratio+0.5)) / ratio
}
