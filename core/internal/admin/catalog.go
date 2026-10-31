package admin

import (
	"errors"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"inflacao-real/core/internal/catalog"
	"inflacao-real/core/internal/equivalence"
)

func (owner handler) candidates(request *gin.Context) {
	result, err := equivalence.Read(request.Request.Context(), owner.database)
	if err != nil {
		fail(request, 500, "unavailable")
		return
	}
	request.JSON(200, result)
}

func (owner handler) products(request *gin.Context) {
	products, err := equivalence.Load(request.Request.Context(), owner.database)
	if err != nil {
		fail(request, 500, "unavailable")
		return
	}
	result := []equivalence.Product{}
	query := catalog.NormalizeText(request.Query("q"))
	for _, product := range products {
		result = appendMatchingProduct(result, product, query)
	}
	request.JSON(200, result)
}

func matchesProduct(product equivalence.Product, query string) bool {
	return strings.Contains(catalog.NormalizeText(product.Name), query) || strconv.FormatInt(product.ID, 10) == query
}

func (owner handler) product(request *gin.Context) {
	identity, err := strconv.ParseInt(request.Param("id"), 10, 64)
	if err != nil {
		fail(request, 400, "invalid_product")
		return
	}
	products, err := equivalence.Load(request.Request.Context(), owner.database)
	if err != nil {
		fail(request, 500, "unavailable")
		return
	}
	index := productsByIdentity(products)
	product, found := index[identity]
	if found {
		request.JSON(200, product)
		return
	}
	fail(request, 404, "product_not_found")
}

func (owner handler) review(request *gin.Context) {
	var decision equivalence.Decision
	if err := request.ShouldBindJSON(&decision); err != nil {
		fail(request, 400, "invalid_request")
		return
	}
	err := equivalence.Apply(request.Request.Context(), owner.database, owner.username, decision)
	reviewResponse(request, err)
}

func reviewResponse(request *gin.Context, err error) {
	if errors.Is(err, equivalence.ErrStale) {
		fail(request, 409, "stale_review")
		return
	}
	if errors.Is(err, equivalence.ErrInvalid) {
		fail(request, 422, "invalid_decision")
		return
	}
	if err != nil {
		fail(request, 500, "unavailable")
		return
	}
	request.Status(204)
}

func (owner handler) group(request *gin.Context) {
	identity, err := strconv.ParseInt(request.Param("id"), 10, 64)
	if err != nil {
		fail(request, 400, "invalid_group")
		return
	}
	snapshot, err := equivalence.Read(request.Request.Context(), owner.database)
	if err != nil {
		fail(request, 500, "unavailable")
		return
	}
	members := groupMembers(snapshot.Products, identity)
	if len(members) == 0 {
		fail(request, 404, "group_not_found")
		return
	}
	request.JSON(200, gin.H{"revision": snapshot.Revision, "representative": identity, "members": members, "audit": snapshot.Audit})
}
func groupMembers(products []equivalence.Product, identity int64) []equivalence.Product {
	members := []equivalence.Product{}
	for _, product := range products {
		members = appendGroupMember(members, product, identity)
	}
	return members
}

func productsByIdentity(products []equivalence.Product) map[int64]equivalence.Product {
	index := map[int64]equivalence.Product{}
	for _, product := range products {
		index[product.ID] = product
	}
	return index
}
func appendGroupMember(products []equivalence.Product, product equivalence.Product, identity int64) []equivalence.Product {
	if product.Representative == identity {
		return append(products, product)
	}
	return products
}
func appendMatchingProduct(products []equivalence.Product, product equivalence.Product, query string) []equivalence.Product {
	if matchesProduct(product, query) {
		return append(products, product)
	}
	return products
}
