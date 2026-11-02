package history

import "inflacao-real/core/internal/ingestion/document"

func offerState(offer document.Offer) map[string]any {
	conditions := offer.Conditions
	return map[string]any{
		"price_amount": offer.Amount, "price_currency": offer.Currency,
		"price_basis": offer.Basis, "reference_amount": offer.ReferenceAmount,
		"payment_method": conditions.PaymentMethod, "installments": conditions.Installments,
		"membership_required": conditions.MembershipRequired,
		"membership_program":  conditions.MembershipProgram, "coupon_required": conditions.CouponRequired,
		"minimum_quantity": conditions.MinimumQuantity, "condition_description": conditions.Description,
	}
}

func measurementState(observation document.Observation, productID int64, offer document.Offer) map[string]any {
	state := offerState(offer)
	state["product_id"], state["availability"] = productID, observation.Availability
	state["offer_count"] = len(observation.Offers)
	location := observation.Context
	state["seller_id"], state["store_id"] = location.SellerID, location.StoreID
	state["postal_code"], state["city"] = location.PostalCode, location.City
	state["state"], state["channel"] = location.State, location.Channel
	applyPackage(state, observation)
	return state
}

func applyPackage(state map[string]any, observation document.Observation) {
	product := observation.Product
	if product.Package == nil {
		return
	}
	size := product.Package
	state["package_quantity"], state["package_unit"], state["package_count"] = size.Quantity, size.Unit, size.Count
}

func measurementStates(observation document.Observation, productID int64) []map[string]any {
	offers := observation.Offers
	if len(offers) == 0 {
		offers = []document.Offer{{}}
	}
	states := make([]map[string]any, 0, len(offers))
	for _, offer := range offers {
		states = append(states, measurementState(observation, productID, offer))
	}
	return states
}
