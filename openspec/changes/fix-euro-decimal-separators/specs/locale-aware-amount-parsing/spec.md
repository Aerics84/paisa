## ADDED Requirements

### Requirement: Ledger parsing preserves comma-decimal precision
The system SHALL parse ledger numeric tokens with comma decimal separators as decimal values when the token structure represents a decimal quantity or price, including high-precision quantities.

#### Scenario: High-precision quantity uses comma decimal separator
- **WHEN** ledger ingestion parses a posting quantity such as `2,707819 "EWG2LD"`
- **THEN** the parsed quantity SHALL be `2.707819`
- **AND** downstream valuation SHALL use the parsed decimal quantity instead of a comma-stripped integer

#### Scenario: Mixed separators represent European formatted prices
- **WHEN** ledger ingestion parses a value such as `10.005,05 EUR`
- **THEN** the parsed numeric value SHALL be `10005.05`

#### Scenario: Existing thousand-separated values remain valid
- **WHEN** ledger ingestion parses a value such as `100,000 EUR`
- **THEN** the parsed numeric value SHALL remain `100000`

### Requirement: Import helpers normalize localized numeric strings
The system SHALL normalize localized numeric strings in import/template helpers so generated ledger values preserve decimal meaning for comma-decimal inputs.

#### Scenario: Helper normalizes comma-decimal amount
- **WHEN** an import helper receives `92,33`
- **THEN** it SHALL normalize the value to `92.33`

#### Scenario: Helper normalizes high-precision comma-decimal quantity
- **WHEN** an import helper receives `2,707819`
- **THEN** it SHALL normalize the value to `2.707819`
