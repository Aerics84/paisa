interface PriceSource {
  commodity_type: string;
  commodity_id: string;
}

// Prices written in the ledger (`P` directives) are tagged "unknown". When a
// commodity also has prices from a configured provider, the newest price can be
// a ledger one, so show the provider's type and id instead.
export function preferredSource<T extends PriceSource>(history: T[]): T | undefined {
  return history.find((price) => price.commodity_type !== "unknown") ?? history[0];
}
