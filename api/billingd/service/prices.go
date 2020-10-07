package service

import (
	stripe "github.com/stripe/stripe-go/v72"
	stripec "github.com/stripe/stripe-go/v72/client"
)

func setupStoredData(client *stripec.API) (string, error) {
	product, err := client.Products.New(&stripe.ProductParams{
		Name:      stripe.String("Stored Data"),
		UnitLabel: stripe.String("50 MiB"),
	})
	if err != nil {
		return "", err
	}
	price, err := client.Prices.New(&stripe.PriceParams{
		Currency: stripe.String(string(stripe.CurrencyUSD)),
		Product:  stripe.String(product.ID),
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
	return price.ID, nil
}

func setupNetworkEgress(client *stripec.API) (string, error) {
	product, err := client.Products.New(&stripe.ProductParams{
		Name:      stripe.String("Network Egress"),
		UnitLabel: stripe.String("100 MiB"),
	})
	if err != nil {
		return "", err
	}
	price, err := client.Prices.New(&stripe.PriceParams{
		Currency: stripe.String(string(stripe.CurrencyUSD)),
		Product:  stripe.String(product.ID),
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
	return price.ID, nil
}

func setupInstanceReads(client *stripec.API) (string, error) {
	product, err := client.Products.New(&stripe.ProductParams{
		Name:      stripe.String("Instance Reads"),
		UnitLabel: stripe.String("10,000"),
	})
	if err != nil {
		return "", err
	}
	price, err := client.Prices.New(&stripe.PriceParams{
		Currency: stripe.String(string(stripe.CurrencyUSD)),
		Product:  stripe.String(product.ID),
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
	return price.ID, nil
}

func setupInstanceWrites(client *stripec.API) (string, error) {
	product, err := client.Products.New(&stripe.ProductParams{
		Name:      stripe.String("Instance Writes"),
		UnitLabel: stripe.String("5,000"),
	})
	if err != nil {
		return "", err
	}
	price, err := client.Prices.New(&stripe.PriceParams{
		Currency: stripe.String(string(stripe.CurrencyUSD)),
		Product:  stripe.String(product.ID),
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
	return price.ID, nil
}
