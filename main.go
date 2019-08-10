package main

import (
	"strings"
	"time"
	"fmt"
	"net/http"
	"net/url"
	"log"
	"strconv"
	"encoding/json"
	"crypto/tls"
	"github.com/PuerkitoBio/goquery"
	"context"
	"github.com/aws/aws-lambda-go/lambda"
)

type GatewayProxyEvent struct {
        Path string `json:"path"`
        Parameters map[string]string `json:"pathParameters"`
}

type Response struct {
        Body string `json:"body"`
}

type Prestacao struct{
	NumParcela int `json:"parcela"`
	Vencimento time.Time `json:"dataVencimento"`
	Valor float64 `json:"valor"`
	Multa float64 `json:"multa"`
	Total float64 `json:"total"`
}

func BrlToFloat(s string) (float64, error) {
	s = strings.ReplaceAll(s, ".", "")
	s = strings.ReplaceAll(s, ",", ".")
	return strconv.ParseFloat(s, 64)
}

func HandleRequest(ctx context.Context, event GatewayProxyEvent) (Response, error) {
	params := event.Parameters
	prestacoes := GetPrestacao(params["contrato"], params["cpf"], params["diaNascimento"])
	json, err := json.Marshal(prestacoes)
	return Response{
			Body: string(json),
		}, err
}

func GetPrestacao(contratoNum string, cpfNum string, diaNascimento string) ([]Prestacao) {
	// fmt.Printf("Starting the request\n")
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}

	// fmt.Printf("%s - %s - %s\n", contratoNum, cpfNum, diaNascimento)
	var prestacoes []Prestacao

	params := url.Values{
			"versao":{"index.asp-rme_000057-007-22/11/2013-172.20.63.89"},
			"txtIdentificacao": {contratoNum},
			"txtCpfCgc": {cpfNum},
			"txtDiaNascMutuario": {diaNascimento},
			"txtNumeroSorteado": {"5"},
			"txtContadorEnvio": nil,
			"txtTitulo": {"Dia+Nascimento+do+Mutuário"},
			"txtContErroSorteiaNovamente": nil,
			"txtCampo1Sorteado": nil, 
			"txtCampo2Sorteado": nil,
			"txtCampo3Sorteado": nil,
	}

	req, err := http.NewRequest("POST", "https://www1.caixa.gov.br/servico/habitacao/asp/login.asp", strings.NewReader(params.Encode()))
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("Cookie", "security=true;")
	req.Header.Add("Host", "www1.caixa.gov.br")
	req.Header.Add("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_14_5) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/75.0.3770.100 Safari/537.36")

	if err != nil {
		log.Fatal(err)
	}

	resp, err := http.DefaultClient.Do(req)	

	defer resp.Body.Close()
	
	if err != nil {
		fmt.Printf("%s", "Error while loging")
		log.Fatal(err)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)

	if err != nil {
		log.Fatal(err)
	}

	controleNum, controlerError := doc.Find("input[name=txtControle]").Attr("value")

	if !controlerError {
		log.Fatal("Error while getting the controle number")
	}

	params = url.Values{
		"txtIdentificacao": {contratoNum},
		"txtCpfCgc": {cpfNum},
		"txtCliente": {""},
		"txtNomeMutuario": {""},
		"txtControle": {controleNum},
		"txtSequencia": {"<#formStringSequencia>"},
		"txtCredor": {"<#formStringCredor>"},
	}

	reqPrestacao, err := http.NewRequest("POST", "https://www1.caixa.gov.br/servico/habitacao/asp/prestacao.asp", strings.NewReader(params.Encode()))
	// fmt.Printf("\n\n%s\n\n", buf.String())
	reqPrestacao.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	reqPrestacao.Header.Add("Cookie", "security=true;")
	reqPrestacao.Header.Add("Host", "www1.caixa.gov.br")
	reqPrestacao.Header.Add("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_14_5) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/75.0.3770.100 Safari/537.36")
	reqPrestacao.Header.Add("Session", "security=true;")


	respPrestacao, err := http.DefaultClient.Do(reqPrestacao)

	if err != nil {
		fmt.Printf("%s\n", "")
		log.Fatal(err)
	}

	defer respPrestacao.Body.Close()

	// Load the HTML document
	doc, err = goquery.NewDocumentFromReader(respPrestacao.Body)
	if err != nil {
		log.Fatal(err)
	}
	// Find the review items
	doc.Find(".dados_contrato tr").Each(func(i int, s *goquery.Selection) {
		if i == 0 {
			return 
		}

		row := s.Find("td")
		dateArr := strings.Split(row.Eq(1).Text(), "/")
		year, err := strconv.Atoi(dateArr[2])
		monthInt, err := strconv.Atoi(dateArr[1])
		month := time.Month(monthInt)
		day, err := strconv.Atoi(dateArr[0])
		vencimento := time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
		// fmt.Printf("%s\n", vencimento)

		numParcelaInt, err := strconv.Atoi(row.Eq(0).Text())

		if err != nil {
			log.Fatal(err)
		}

		valorFloat, err := BrlToFloat(row.Eq(2).Text())
		multaFloat, err := BrlToFloat(row.Eq(4).Text())
		totalFloat, err := BrlToFloat(row.Eq(5).Text())
		
		prestacoes = append(prestacoes, Prestacao {
			NumParcela: numParcelaInt,
			Vencimento: vencimento,
			Valor: valorFloat,
			Multa: multaFloat,
			Total: totalFloat,
		})
		
	})

	return prestacoes

}

func main() {
        lambda.Start(HandleRequest)
}

