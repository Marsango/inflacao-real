// Package selections defines curated filters over persisted monitoring labels.
package selections

import "slices"

type Group struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Description string `json:"description"`
	targets     []string
}

var groups = []Group{
	{ID: "cesta-basica", Label: "Cesta básica",
		Description: "Seleção de arroz branco tipo 1, feijão, leite, café moído, açúcar refinado ou cristal, óleo de soja, carne bovina de primeira, farinhas de trigo e mandioca, batata, tomate, pão francês, banana e manteiga. Cada produto tem peso igual; não representa o custo nem o índice de uma cesta básica oficial.",
		targets:     []string{"arroz-branco-tipo-1", "feijao", "leite", "cafe-moido", "acucar-refinado", "acucar-cristal", "oleo-soja", "carne-bovina-primeira", "farinha-trigo", "farinha-mandioca", "batata", "tomate", "pao-frances", "banana", "manteiga"}},
	{ID: "hortifruti", Label: "Hortifruti",
		Description: "Frutas, verduras, legumes e raízes frescos monitorados: banana, maçã, laranja, mamão, limão, melancia, batata, tomate, cebola, alho, cenoura, alface, repolho, beterraba, chuchu, abobrinha, mandioca e batata-doce.",
		targets:     []string{"banana", "maca", "laranja", "mamao", "limao", "melancia", "batata", "tomate", "cebola", "alho", "cenoura", "alface", "repolho", "beterraba", "chuchu", "abobrinha", "mandioca", "batata-doce"}},
	{ID: "doces", Label: "Doces",
		Description: "Chocolates em barra, bombons, balas, chicletes, biscoitos doces, recheados e wafers, sorvetes e leite condensado monitorados.",
		targets:     []string{"chocolate-barra", "bombom", "bala", "chiclete", "biscoito-doce", "biscoito-recheado", "biscoito-wafer", "sorvete", "leite-condensado"}},
	{ID: "carnes", Label: "Carnes",
		Description: "Cortes bovinos, suínos, frango e filé de tilápia monitorados. Não inclui embutidos, carnes temperadas ou pratos prontos.",
		targets:     []string{"carne-bovina-primeira", "carne-bovina-acem", "carne-bovina-musculo", "pernil-suino", "lombo-suino", "frango-peito", "frango-coxa", "frango-sobrecoxa", "frango-inteiro", "frango-coxa-sobrecoxa", "file-tilapia"}},
}

func All() []Group { return append([]Group{}, groups...) }

func Find(identity string) (Group, bool) {
	position := slices.IndexFunc(groups, func(group Group) bool { return group.ID == identity })
	if position < 0 {
		return Group{}, false
	}
	return groups[position], true
}

func Valid(identity string) bool {
	_, found := Find(identity)
	return identity == "" || found
}

func Targets(identity string) []string {
	group, _ := Find(identity)
	return append([]string{}, group.targets...)
}
