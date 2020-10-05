package service

import (
	stripe "github.com/stripe/stripe-go/v72"
	"github.com/stripe/stripe-go/v72/price"
	"github.com/stripe/stripe-go/v72/product"
)

func setupStoredData() (string, error) {
	pro, err := product.New(&stripe.ProductParams{
		Name:      stripe.String("Stored Data"),
		UnitLabel: stripe.String("50 MiB"),
	})
	if err != nil {
		return "", err
	}
	pri, err := price.New(&stripe.PriceParams{
		Currency: stripe.String(string(stripe.CurrencyUSD)),
		Product:  stripe.String(pro.ID),
		Recurring: &stripe.PriceRecurringParams{
			AggregateUsage: stripe.String(string(stripe.PriceRecurringAggregateUsageLastEver)),
			Interval:       stripe.String(string(stripe.PriceRecurringIntervalDay)),
			IntervalCount:  stripe.Int64(30),
			UsageType:      stripe.String(string(stripe.PriceRecurringUsageTypeMetered)),
		},
		Tiers: []*stripe.PriceTierParams{
			{
				UpTo:       stripe.Int64(500),
				UnitAmount: stripe.Int64(0),
			},
			{
				UpToInf:    stripe.Bool(true),
				UnitAmount: stripe.Int64(1),
			},
		},
		TiersMode:     stripe.String(string(stripe.PriceTiersModeGraduated)),
		BillingScheme: stripe.String(string(stripe.PriceBillingSchemeTiered)),
	})
	if err != nil {
		return "", err
	}
	return pri.ID, nil
}

func setupNetworkEgress() (string, error) {
	pro, err := product.New(&stripe.ProductParams{
		Name:      stripe.String("Network Egress"),
		UnitLabel: stripe.String("100 MiB"),
	})
	if err != nil {
		return "", err
	}
	pri, err := price.New(&stripe.PriceParams{
		Currency: stripe.String(string(stripe.CurrencyUSD)),
		Product:  stripe.String(pro.ID),
		Recurring: &stripe.PriceRecurringParams{
			AggregateUsage: stripe.String(string(stripe.PriceRecurringAggregateUsageSum)),
			Interval:       stripe.String(string(stripe.PriceRecurringIntervalDay)),
			IntervalCount:  stripe.Int64(30),
			UsageType:      stripe.String(string(stripe.PriceRecurringUsageTypeMetered)),
		},
		Tiers: []*stripe.PriceTierParams{
			{
				UpTo:       stripe.Int64(3),
				UnitAmount: stripe.Int64(0),
			},
			{
				UpToInf:    stripe.Bool(true),
				UnitAmount: stripe.Int64(1),
			},
		},
		TiersMode:     stripe.String(string(stripe.PriceTiersModeGraduated)),
		BillingScheme: stripe.String(string(stripe.PriceBillingSchemeTiered)),
	})
	if err != nil {
		return "", err
	}
	return pri.ID, nil
}

func setupInstanceReads() (string, error) {
	pro, err := product.New(&stripe.ProductParams{
		Name:      stripe.String("Instance Reads"),
		UnitLabel: stripe.String("10,000"),
	})
	if err != nil {
		return "", err
	}
	pri, err := price.New(&stripe.PriceParams{
		Currency: stripe.String(string(stripe.CurrencyUSD)),
		Product:  stripe.String(pro.ID),
		Recurring: &stripe.PriceRecurringParams{
			AggregateUsage: stripe.String(string(stripe.PriceRecurringAggregateUsageSum)),
			Interval:       stripe.String(string(stripe.PriceRecurringIntervalDay)),
			IntervalCount:  stripe.Int64(30),
			UsageType:      stripe.String(string(stripe.PriceRecurringUsageTypeMetered)),
		},
		Tiers: []*stripe.PriceTierParams{
			{
				UpTo:       stripe.Int64(1),
				UnitAmount: stripe.Int64(0),
			},
			{
				UpToInf:    stripe.Bool(true),
				UnitAmount: stripe.Int64(1),
			},
		},
		TiersMode:     stripe.String(string(stripe.PriceTiersModeGraduated)),
		BillingScheme: stripe.String(string(stripe.PriceBillingSchemeTiered)),
	})
	if err != nil {
		return "", err
	}
	return pri.ID, nil
}

func setupInstanceWrites() (string, error) {
	pro, err := product.New(&stripe.ProductParams{
		Name:      stripe.String("Instance Writes"),
		UnitLabel: stripe.String("5,000"),
	})
	if err != nil {
		return "", err
	}
	pri, err := price.New(&stripe.PriceParams{
		Currency: stripe.String(string(stripe.CurrencyUSD)),
		Product:  stripe.String(pro.ID),
		Recurring: &stripe.PriceRecurringParams{
			AggregateUsage: stripe.String(string(stripe.PriceRecurringAggregateUsageSum)),
			Interval:       stripe.String(string(stripe.PriceRecurringIntervalDay)),
			IntervalCount:  stripe.Int64(30),
			UsageType:      stripe.String(string(stripe.PriceRecurringUsageTypeMetered)),
		},
		Tiers: []*stripe.PriceTierParams{
			{
				UpTo:       stripe.Int64(1),
				UnitAmount: stripe.Int64(0),
			},
			{
				UpToInf:    stripe.Bool(true),
				UnitAmount: stripe.Int64(1),
			},
		},
		TiersMode:     stripe.String(string(stripe.PriceTiersModeGraduated)),
		BillingScheme: stripe.String(string(stripe.PriceBillingSchemeTiered)),
	})
	if err != nil {
		return "", err
	}
	return pri.ID, nil
}
