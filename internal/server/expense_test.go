package server

import (
	"testing"
	"time"

	"github.com/ananthakumaran/paisa/internal/model/posting"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func hierarchyGraphPosting(transactionID string, date string, account string, amount string) posting.Posting {
	parsed, err := time.Parse("2006-01-02", date)
	if err != nil {
		panic(err)
	}

	value := decimal.RequireFromString(amount)
	return posting.Posting{
		TransactionID: transactionID,
		Date:          parsed,
		Account:       account,
		Commodity:     "EUR",
		Quantity:      value,
		Amount:        value,
	}
}

func findGraphLinkValue(graph Graph, source string, target string) (decimal.Decimal, bool) {
	var sourceID, targetID uint
	var sourceFound, targetFound bool

	for _, node := range graph.Nodes {
		if node.Name == source {
			sourceID, sourceFound = node.ID, true
		}
		if node.Name == target {
			targetID, targetFound = node.ID, true
		}
	}

	if !sourceFound || !targetFound {
		return decimal.Zero, false
	}

	for _, link := range graph.Links {
		if link.Source == sourceID && link.Target == targetID {
			return link.Value, true
		}
	}

	return decimal.Zero, false
}

func sumGraphLinksFrom(graph Graph, account string) decimal.Decimal {
	var accountID uint
	var found bool
	for _, node := range graph.Nodes {
		if node.Name == account {
			accountID, found = node.ID, true
		}
	}
	if !found {
		return decimal.Zero
	}

	total := decimal.Zero
	for _, link := range graph.Links {
		if link.Source == accountID {
			total = total.Add(link.Value)
		}
	}
	return total
}

func sumGraphLinksTo(graph Graph, account string) decimal.Decimal {
	var accountID uint
	var found bool
	for _, node := range graph.Nodes {
		if node.Name == account {
			accountID, found = node.ID, true
		}
	}
	if !found {
		return decimal.Zero
	}

	total := decimal.Zero
	for _, link := range graph.Links {
		if link.Target == accountID {
			total = total.Add(link.Value)
		}
	}
	return total
}

func TestComputeHierarchyGraphBalancesSingleTransaction(t *testing.T) {
	graph := computeHierarchyGraph([]posting.Posting{
		hierarchyGraphPosting("txn-1", "2022-01-01", "Assets:Checking:Bank", "-100"),
		hierarchyGraphPosting("txn-1", "2022-01-01", "Expenses:Food:Groceries", "100"),
	})

	value, ok := findGraphLinkValue(graph, "Assets:Checking:Bank", "Expenses")
	require.True(t, ok, "expected the flow to enter the Expenses hierarchy root")
	require.True(t, value.Equal(decimal.NewFromInt(100)), "expected flow of 100, got %s", value)

	expenseGroupValue, ok := findGraphLinkValue(graph, "Expenses", "Expenses:Food")
	require.True(t, ok, "expected the expense group hierarchy link")
	require.True(t, expenseGroupValue.Equal(decimal.NewFromInt(100)), "expected group link of 100, got %s", expenseGroupValue)

	expenseLeafValue, ok := findGraphLinkValue(graph, "Expenses:Food", "Expenses:Food:Groceries")
	require.True(t, ok, "expected the expense leaf hierarchy link")
	require.True(t, expenseLeafValue.Equal(decimal.NewFromInt(100)), "expected leaf link of 100, got %s", expenseLeafValue)

	require.True(t, sumGraphLinksTo(graph, "Expenses:Food:Groceries").Equal(decimal.NewFromInt(100)))
}

func TestComputeHierarchyGraphHandlesMultipleSourcesAndDestinations(t *testing.T) {
	graph := computeHierarchyGraph([]posting.Posting{
		hierarchyGraphPosting("txn-1", "2022-01-01", "Assets:Checking:Bank", "-100"),
		hierarchyGraphPosting("txn-1", "2022-01-01", "Assets:Checking:Cash", "-50"),
		hierarchyGraphPosting("txn-1", "2022-01-01", "Assets:Equity:VWCE", "80"),
		hierarchyGraphPosting("txn-1", "2022-01-01", "Assets:Equity:VTI", "70"),
	})

	total := decimal.Zero
	for _, link := range graph.Links {
		total = total.Add(link.Value)
	}
	require.True(t, total.Equal(decimal.NewFromInt(150)), "expected all outflows to be paired, got %s", total)

	require.True(t, sumGraphLinksFrom(graph, "Assets:Checking:Bank").Equal(decimal.NewFromInt(100)))
	require.True(t, sumGraphLinksFrom(graph, "Assets:Checking:Cash").Equal(decimal.NewFromInt(50)))
	require.True(t, sumGraphLinksTo(graph, "Assets:Equity:VWCE").Equal(decimal.NewFromInt(80)))
	require.True(t, sumGraphLinksTo(graph, "Assets:Equity:VTI").Equal(decimal.NewFromInt(70)))
}

func TestComputeHierarchyGraphExpandsIncomeAndExpenseHierarchy(t *testing.T) {
	graph := computeHierarchyGraph([]posting.Posting{
		hierarchyGraphPosting("txn-1", "2022-01-01", "Income:Salary:Acme", "-1000"),
		hierarchyGraphPosting("txn-1", "2022-01-01", "Assets:Checking:Bank", "1000"),
	})

	leafToParent, ok := findGraphLinkValue(graph, "Income:Salary:Acme", "Income:Salary")
	require.True(t, ok)
	require.True(t, leafToParent.Equal(decimal.NewFromInt(1000)))

	parentToRoot, ok := findGraphLinkValue(graph, "Income:Salary", "Income")
	require.True(t, ok)
	require.True(t, parentToRoot.Equal(decimal.NewFromInt(1000)))

	rootToAsset, ok := findGraphLinkValue(graph, "Income", "Assets:Checking:Bank")
	require.True(t, ok)
	require.True(t, rootToAsset.Equal(decimal.NewFromInt(1000)))

	_, directLink := findGraphLinkValue(graph, "Income:Salary:Acme", "Assets:Checking:Bank")
	require.False(t, directLink, "hierarchical flows must not bypass their parent accounts")

	total := decimal.Zero
	for _, link := range graph.Links {
		total = total.Add(link.Value)
	}
	require.True(t, total.Equal(decimal.NewFromInt(3000)), "each hierarchy level represents the same income once, got %s", total)
}

func TestComputeHierarchyGraphKeepsCheckingTransfersInternal(t *testing.T) {
	graph := computeHierarchyGraph([]posting.Posting{
		hierarchyGraphPosting("txn-1", "2022-01-01", "Assets:Checking:BankA", "-100"),
		hierarchyGraphPosting("txn-1", "2022-01-01", "Assets:Checking:BankB", "100"),
	})

	require.Len(t, graph.Links, 1, "a transfer between checking accounts is one asset-to-asset flow")

	value, ok := findGraphLinkValue(graph, "Assets:Checking:BankA", "Assets:Checking:BankB")
	require.True(t, ok)
	require.True(t, value.Equal(decimal.NewFromInt(100)))

	for _, node := range graph.Nodes {
		require.NotEqual(t, "Income", node.Name)
		require.NotEqual(t, "Expenses", node.Name)
	}
}

func TestComputeHierarchyGraphSeparatesLoanPrincipalAndInterest(t *testing.T) {
	graph := computeHierarchyGraph([]posting.Posting{
		hierarchyGraphPosting("txn-1", "2022-01-01", "Assets:Checking:Bank", "-1100"),
		hierarchyGraphPosting("txn-1", "2022-01-01", "Liabilities:Loan:Home", "1000"),
		hierarchyGraphPosting("txn-1", "2022-01-01", "Expenses:Interest:HomeLoan", "100"),
	})

	principal, ok := findGraphLinkValue(graph, "Assets:Checking:Bank", "Liabilities:Loan:Home")
	require.True(t, ok, "expected principal repayment to remain a liability flow")
	require.True(t, principal.Equal(decimal.NewFromInt(1000)))

	interest, ok := findGraphLinkValue(graph, "Assets:Checking:Bank", "Expenses")
	require.True(t, ok, "expected interest to remain an expense flow")
	require.True(t, interest.Equal(decimal.NewFromInt(100)))

	interestHierarchy, ok := findGraphLinkValue(graph, "Expenses:Interest", "Expenses:Interest:HomeLoan")
	require.True(t, ok)
	require.True(t, interestHierarchy.Equal(decimal.NewFromInt(100)))

	_, mixed := findGraphLinkValue(graph, "Liabilities:Loan:Home", "Expenses:Interest:HomeLoan")
	require.False(t, mixed, "principal and interest must not be merged into one destination")
}

func TestComputeHierarchyGraphIncludesInvestmentAndLiabilityFlows(t *testing.T) {
	graph := computeHierarchyGraph([]posting.Posting{
		hierarchyGraphPosting("txn-1", "2022-01-01", "Assets:Checking:Bank", "-1000"),
		hierarchyGraphPosting("txn-1", "2022-01-01", "Assets:Equity:VWCE", "900"),
		hierarchyGraphPosting("txn-1", "2022-01-01", "Expenses:Charges:Brokerage", "100"),
		hierarchyGraphPosting("txn-2", "2022-01-02", "Assets:Checking:Bank", "-300"),
		hierarchyGraphPosting("txn-2", "2022-01-02", "Liabilities:CreditCard:Visa", "300"),
	})

	investment, ok := findGraphLinkValue(graph, "Assets:Checking:Bank", "Assets:Equity:VWCE")
	require.True(t, ok, "investment purchases must stay traceable as asset flows")
	require.True(t, investment.Equal(decimal.NewFromInt(900)))

	charge, ok := findGraphLinkValue(graph, "Assets:Checking:Bank", "Expenses")
	require.True(t, ok)
	require.True(t, charge.Equal(decimal.NewFromInt(100)))

	chargeHierarchy, ok := findGraphLinkValue(graph, "Expenses:Charges", "Expenses:Charges:Brokerage")
	require.True(t, ok)
	require.True(t, chargeHierarchy.Equal(decimal.NewFromInt(100)))

	liability, ok := findGraphLinkValue(graph, "Assets:Checking:Bank", "Liabilities:CreditCard:Visa")
	require.True(t, ok)
	require.True(t, liability.Equal(decimal.NewFromInt(300)))
}

func TestComputeHierarchyGraphPreservesMinorCurrencyAmounts(t *testing.T) {
	graph := computeHierarchyGraph([]posting.Posting{
		hierarchyGraphPosting("txn-1", "2022-01-01", "Assets:Checking:Bank", "-0.05"),
		hierarchyGraphPosting("txn-1", "2022-01-01", "Expenses:Snacks:Vending", "0.05"),
	})

	value, ok := findGraphLinkValue(graph, "Assets:Checking:Bank", "Expenses")
	require.True(t, ok, "a valid 0.05 flow must not be discarded")
	require.True(t, value.Equal(decimal.RequireFromString("0.05")), "expected a flow of 0.05, got %s", value)

	leafValue, ok := findGraphLinkValue(graph, "Expenses:Snacks", "Expenses:Snacks:Vending")
	require.True(t, ok, "the hierarchy path of a cent-level flow must be preserved")
	require.True(t, leafValue.Equal(decimal.RequireFromString("0.05")))
}
