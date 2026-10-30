package webapi

import (
	"errors"
	"net/url"
	"strconv"
	"time"
)

func identity(value string) (int64, error) {
	result, err := strconv.ParseInt(value, 10, 64)
	if err != nil || result <= 0 || strconv.FormatInt(result, 10) != value {
		return 0, errors.New("invalid identity")
	}
	return result, nil
}

func historyRequest(query url.Values) (HistoryRequest, error) {
	request := HistoryRequest{query.Get("product"), query.Get("start"), query.Get("end")}
	if err := validProduct(request.Product); err != nil {
		return request, err
	}
	start, startError := time.Parse("2006-01-02", request.Start)
	end, endError := time.Parse("2006-01-02", request.End)
	if startError != nil || endError != nil || start.Year() < 1 || end.Before(start) || end.Sub(start) > 366*24*time.Hour {
		return request, errors.New("invalid period")
	}
	return request, nil
}

func pagination(query url.Values) (int64, int, error) {
	after, limit := int64(0), 100
	var err error
	if query.Has("after") {
		after, err = identity(query.Get("after"))
	}
	if err != nil {
		return 0, 0, err
	}
	if query.Has("limit") {
		limit, err = strconv.Atoi(query.Get("limit"))
	}
	if err != nil || limit < 1 || limit > 200 {
		return 0, 0, errors.New("invalid pagination")
	}
	return after, limit, nil
}

func validProduct(product string) error {
	if product == "global" {
		return nil
	}
	_, err := identity(product)
	return err
}
