## ADDED Requirements

### Requirement: Metal providers SHALL declare explicit regional and normalization behavior
The system SHALL expose metal price providers with metadata that makes their geographic scope, supported metal codes, output currency, and normalized unit explicit to users and tests.

#### Scenario: User inspects provider metadata
- **WHEN** the system returns the list of price providers
- **THEN** each metal provider SHALL describe its regional scope
- **AND** each metal provider SHALL describe the normalized output currency
- **AND** each metal provider SHALL describe the normalized output unit

### Requirement: European metal pricing SHALL persist prices as EUR per gram
The system SHALL support a European metal price provider that fetches supported metal prices and persists them as daily `EUR per gram` unit prices for downstream valuation.

#### Scenario: Gold price is ingested for a European configuration
- **WHEN** the European metal provider fetches a daily price for `gold-999`
- **THEN** the stored `price.Price` value SHALL represent the price of one gram of gold in EUR for that day

#### Scenario: Silver price is ingested for a European configuration
- **WHEN** the European metal provider fetches a daily price for `silver-999`
- **THEN** the stored `price.Price` value SHALL represent the price of one gram of silver in EUR for that day

### Requirement: Metal valuation SHALL use normalized per-gram unit prices without downstream special cases
The system SHALL value metal holdings using the same generic market valuation flow as other commodities once the provider has normalized the unit price.

#### Scenario: Market value is calculated for a metal holding
- **WHEN** a report or service requests the market value of a metal holding with quantity `Q`
- **THEN** the system SHALL calculate the value as `Q * storedUnitPrice`
- **AND** the system SHALL NOT require metal-specific conversion logic in downstream valuation code

### Requirement: India and EU metal providers SHALL remain distinguishable
The system SHALL preserve separate provider identities for India-specific and Europe-specific metal price sources so existing configurations do not silently change semantics.

#### Scenario: Existing India configuration is loaded
- **WHEN** a configuration references the existing India metal provider
- **THEN** the system SHALL continue to resolve that provider without remapping it to the European provider

#### Scenario: European user selects a metal provider
- **WHEN** a user configures a metal commodity for European valuation
- **THEN** the user SHALL be able to select a provider that is explicitly described as EUR-based and Europe-oriented
