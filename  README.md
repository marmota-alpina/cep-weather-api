# CEP Weather API (Go)

> API desenvolvida em Go que recebe um CEP brasileiro, identifica a cidade correspondente e retorna a temperatura atual em Celsius, Fahrenheit e Kelvin. A aplicação é projetada para ser executada via Docker e implantada no Google Cloud Run.

## Índice

* [Visão Geral](#visão-geral)
* [Endpoints da API](#endpoints-da-api)
* [Fórmulas de Conversão](#fórmulas-de-conversão)
* [Pré-requisitos (Uso Local)](#pré-requisitos-uso-local)
* [Instalação e Execução (Local com Docker)](#instalação-e-execução-local-com-docker)
* [Testes Automatizados](#testes-automatizados)
* [Acesso ao Serviço Implantado (Google Cloud Run)](#acesso-ao-serviço-implantado-google-cloud-run)
* [Variáveis de Ambiente](#variáveis-de-ambiente)
* [Tecnologias Utilizadas](#tecnologias-utilizadas)
* [Estrutura do Projeto (Sugestão)](#estrutura-do-projeto-sugestão)

## Visão Geral

Este sistema recebe um Código de Endereçamento Postal (CEP) brasileiro válido de 8 dígitos. Utilizando a API [ViaCEP](https://viacep.com.br/) (ou similar), ele busca a cidade associada ao CEP fornecido. Em seguida, consulta a API [WeatherAPI](https://www.weatherapi.com/) (ou similar) para obter a temperatura atual dessa cidade. Por fim, a API retorna a temperatura convertida para as escalas Celsius, Fahrenheit e Kelvin.

## Endpoints da API

### Obter Clima por CEP

* **Método:** `GET`
* **Endpoint:** `/weather/{cep}`
* **Parâmetros da URL:**
    * `cep` (string, obrigatório): O CEP brasileiro de 8 dígitos (somente números). Ex: `01001000`.
* **Resposta de Sucesso:**
    * **Código HTTP:** `200 OK`
    * **Content-Type:** `application/json`
    * **Response Body:**
        ```json
        {
          "temp_C": 21.0,
          "temp_F": 69.8,
          "temp_K": 294.0
        }
        ```
      *(Os valores são exemplos)*
* **Respostas de Erro:**
    * **Cenário:** CEP com formato inválido (não contém 8 dígitos numéricos).
        * **Código HTTP:** `422 Unprocessable Entity`
        * **Content-Type:** `text/plain`
        * **Response Body:** `invalid zipcode`
    * **Cenário:** CEP válido no formato, mas não encontrado na base do ViaCEP (ou serviço similar).
        * **Código HTTP:** `404 Not Found`
        * **Content-Type:** `text/plain`
        * **Response Body:** `can not find zipcode`
    * **Cenário:** Erro interno ao consultar APIs externas ou processar a requisição.
        * **Código HTTP:** `500 Internal Server Error`
        * **Response Body:** [Mensagem de erro interna, se aplicável]

## Fórmulas de Conversão

As seguintes fórmulas são utilizadas para converter a temperatura (obtida primariamente em Celsius):

* Celsius para Fahrenheit: $F = C \times 1.8 + 32$
* Celsius para Kelvin: $K = C + 273$

Onde:
* $C$ = Temperatura em graus Celsius
* $F$ = Temperatura em graus Fahrenheit
* $K$ = Temperatura em Kelvin

## Pré-requisitos (Uso Local)

* [Docker](https://www.docker.com/products/docker-desktop/)
* [Docker Compose](https://docs.docker.com/compose/install/) (geralmente incluído no Docker Desktop)
* [Git](https://git-scm.com/)
* Uma **chave de API válida** do [WeatherAPI](https://www.weatherapi.com/). Você precisará se registrar (o plano gratuito é suficiente) e obter sua chave.

## Instalação e Execução (Local com Docker)

Siga os passos abaixo para configurar e executar a aplicação localmente usando Docker:

1.  **Clone o repositório:**
    ```bash
    git clone [https://github.com/marmota-alpina/cep-weather-api.git](https://github.com/marmota-alpina/cep-weather-api.git)
    cd cep-weather-api
    ```

2.  **Configure as Variáveis de Ambiente:**
    * Adicione a seguinte variável, substituindo `SUA_CHAVE_AQUI` pela sua chave da WeatherAPI:
        ```dotenv
        WEATHER_API_KEY=SUA_CHAVE_AQUI
        ```

3.  **Construa a Imagem Docker e Inicie o Container:**
    ```bash
    docker-compose build
    docker-compose up -d # O '-d' executa em modo detached (background)
    ```

4.  **Acesse a API:**
    A aplicação estará rodando e acessível em `http://localhost:8080` (ou a porta que você mapeou no `docker-compose.yml`).

    **Exemplos com `curl`:**
    ```bash
    # Requisição com sucesso
    curl http://localhost:8080/weather/01001000

    # CEP com formato inválido
    curl -i http://localhost:8080/weather/12345

    # CEP não encontrado
    curl -i http://localhost:8080/weather/99999999
    ```
    *O `-i` no curl exibe os cabeçalhos HTTP, ajudando a ver o status code.*

5.  **Para Parar a Aplicação:**
    ```bash
    docker-compose down
    ```

## Testes Automatizados

Para executar os testes automatizados definidos no projeto, utilize o comando a seguir:

```bash
go test

```` 