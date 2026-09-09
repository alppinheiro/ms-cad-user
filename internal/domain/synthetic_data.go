package domain

// ===========================================================================
// DADOS-BASE DO GERADOR SINTÉTICO (somente para carga de estudo)
// ---------------------------------------------------------------------------
// Este arquivo NÃO é regra de negócio — é o "dicionário" de dados realistas
// usados para popular o Kafka/Mongo em massa. As escolhas aqui existem para
// que as consultas de estudo façam sentido:
//   - Todas as 27 UFs com cidades reais (capital + principais) → permite
//     agrupar/filtrar por UF e cidade com volume distribuído;
//   - DDD coerente com a cidade sorteada e faixa de CEP coerente com a UF →
//     endereço e telefone não se contradizem (dado "sujo" atrapalharia o estudo);
//   - Nomes, sobrenomes, bairros e logradouros em pt-BR para buscas textuais.
//
// Representação do CEP: guardamos o PREFIXO (5 dígitos) como int e formatamos
// com %05d na hora de montar o endereço. Ex.: 1000 vira "01000", 13000 vira
// "13000". Assim as faixas por cidade ficam fáceis de escrever e ler.
// ===========================================================================

// cidade representa uma cidade sintética com DDD e faixa de prefixo de CEP.
// cepMin/cepMax delimitam a faixa de prefixos (5 dígitos) válida p/ a cidade.
type cidade struct {
	nome   string
	ddd    int
	cepMin int
	cepMax int
}

// UFs em ordem fixa para amostragem uniforme.
var ufs = []string{
	"AC", "AL", "AP", "AM", "BA", "CE", "DF", "ES", "GO", "MA",
	"MT", "MS", "MG", "PA", "PB", "PR", "PE", "PI", "RJ", "RN",
	"RS", "RO", "RR", "SC", "SP", "SE", "TO",
}

// citiesByUF mapeia cada UF às cidades (capital + principais).
var citiesByUF = map[string][]cidade{
	"AC": {{nome: "Rio Branco", ddd: 68, cepMin: 69900, cepMax: 69999}},
	"AL": {{nome: "Maceió", ddd: 82, cepMin: 57000, cepMax: 57099}},
	"AP": {{nome: "Macapá", ddd: 96, cepMin: 68900, cepMax: 68999}},
	"AM": {{nome: "Manaus", ddd: 92, cepMin: 69000, cepMax: 69099}},
	"BA": {
		{nome: "Salvador", ddd: 71, cepMin: 40000, cepMax: 41999},
		{nome: "Feira de Santana", ddd: 75, cepMin: 44000, cepMax: 44199},
	},
	"CE": {{nome: "Fortaleza", ddd: 85, cepMin: 60000, cepMax: 60899}},
	"DF": {{nome: "Brasília", ddd: 61, cepMin: 70000, cepMax: 71699}},
	"ES": {{nome: "Vitória", ddd: 27, cepMin: 29000, cepMax: 29099}},
	"GO": {{nome: "Goiânia", ddd: 62, cepMin: 74000, cepMax: 74899}},
	"MA": {{nome: "São Luís", ddd: 98, cepMin: 65000, cepMax: 65099}},
	"MT": {{nome: "Cuiabá", ddd: 65, cepMin: 78000, cepMax: 78099}},
	"MS": {{nome: "Campo Grande", ddd: 67, cepMin: 79000, cepMax: 79099}},
	"MG": {
		{nome: "Belo Horizonte", ddd: 31, cepMin: 30000, cepMax: 31999},
		{nome: "Uberlândia", ddd: 34, cepMin: 38400, cepMax: 38499},
	},
	"PA": {{nome: "Belém", ddd: 91, cepMin: 66000, cepMax: 66999}},
	"PB": {{nome: "João Pessoa", ddd: 83, cepMin: 58000, cepMax: 58099}},
	"PR": {
		{nome: "Curitiba", ddd: 41, cepMin: 80000, cepMax: 82999},
		{nome: "Londrina", ddd: 43, cepMin: 86000, cepMax: 86099},
	},
	"PE": {{nome: "Recife", ddd: 81, cepMin: 50000, cepMax: 52999}},
	"PI": {{nome: "Teresina", ddd: 86, cepMin: 64000, cepMax: 64099}},
	"RJ": {
		{nome: "Rio de Janeiro", ddd: 21, cepMin: 20000, cepMax: 23799},
		{nome: "Niterói", ddd: 21, cepMin: 24000, cepMax: 24399},
	},
	"RN": {{nome: "Natal", ddd: 84, cepMin: 59000, cepMax: 59099}},
	"RS": {
		{nome: "Porto Alegre", ddd: 51, cepMin: 90000, cepMax: 91999},
		{nome: "Caxias do Sul", ddd: 54, cepMin: 95000, cepMax: 95099},
	},
	"RO": {{nome: "Porto Velho", ddd: 69, cepMin: 76800, cepMax: 76899}},
	"RR": {{nome: "Boa Vista", ddd: 95, cepMin: 69300, cepMax: 69399}},
	"SC": {
		{nome: "Florianópolis", ddd: 48, cepMin: 88000, cepMax: 88099},
		{nome: "Joinville", ddd: 47, cepMin: 89200, cepMax: 89299},
	},
	"SP": {
		{nome: "São Paulo", ddd: 11, cepMin: 1000, cepMax: 5699}, // 01000..05699
		{nome: "São Bernardo do Campo", ddd: 11, cepMin: 9700, cepMax: 9799},
		{nome: "Santos", ddd: 13, cepMin: 11000, cepMax: 11099},
		{nome: "Ribeirão Preto", ddd: 16, cepMin: 14000, cepMax: 14099},
		{nome: "Sorocaba", ddd: 15, cepMin: 18000, cepMax: 18099},
		{nome: "Campinas", ddd: 19, cepMin: 13000, cepMax: 13099},
	},
	"SE": {{nome: "Aracaju", ddd: 79, cepMin: 49000, cepMax: 49099}},
	"TO": {{nome: "Palmas", ddd: 63, cepMin: 77000, cepMax: 77099}},
}

// Bases de nomes comuns brasileiros.
var firstNamesM = []string{
	"João", "Pedro", "Lucas", "Gabriel", "Mateus", "Rafael", "Thiago",
	"Bruno", "Diego", "Felipe", "Gustavo", "Henrique", "Igor", "Leonardo",
	"Marcos", "Matheus", "Paulo", "Renato", "Rodrigo", "Samuel", "Vinícius",
	"André", "Caio", "Daniel", "Eduardo", "Fábio",
}

var firstNamesF = []string{
	"Maria", "Ana", "Beatriz", "Camila", "Carolina", "Débora", "Fernanda",
	"Gabriela", "Isabela", "Juliana", "Larissa", "Letícia", "Luana", "Mariana",
	"Natália", "Patrícia", "Rafaela", "Raquel", "Renata", "Sabrina", "Tatiane",
	"Vanessa", "Aline", "Bianca", "Carla", "Elaine",
}

var lastNames = []string{
	"da Silva", "Santos", "Oliveira", "Souza", "Rodrigues", "Ferreira",
	"Alves", "Pereira", "Lima", "Gomes", "Costa", "Ribeiro", "Martins",
	"Carvalho", "Almeida", "Lopes", "Soares", "Fernandes", "Vieira", "Barbosa",
	"Rocha", "Dias", "Nascimento", "Moreira", "Cardoso", "Teixeira",
}

var emailDomains = []string{
	"gmail.com", "hotmail.com", "outlook.com", "yahoo.com.br",
	"uol.com.br", "bol.com.br", "icloud.com", "proton.me", "live.com",
}

var streetKinds = []string{
	"Rua", "Avenida", "Alameda", "Travessa", "Praça", "Rodovia",
}

var streets = []string{
	"das Flores", "Brasil", "São João", "Paulista", "Amazonas",
	"Getúlio Vargas", "Tiradentes", "das Palmeiras", "XV de Novembro",
	"Rio Branco", "Sete de Setembro", "Dom Pedro II", "Osvaldo Cruz",
	"Santos Dumont", "Bandeirantes", "Ipiranga", "Nove de Julho", "da Paz",
	"Araújo Porto Alegre", "Gonçalves Dias",
}

var bairros = []string{
	"Centro", "Jardim América", "Vila Nova", "Bela Vista", "Santa Cecília",
	"Alto da Boa Vista", "Campo Belo", "Moema", "Itaim Bibi", "Pinheiros",
	"Tatuapé", "Penha", "Santana", "Barra Funda", "Saúde", "Ipiranga",
	"Vila Mariana", "Liberdade", "Consolação", "Botafogo", "Boa Viagem",
	"Copacabana", "Água Verde", "Batel",
}
