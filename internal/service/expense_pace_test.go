package service

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func expensePaceAsOf(day int) time.Time {
	return time.Date(2026, time.September, day, 0, 0, 0, 0, time.UTC)
}

func expensePaceMoney(v int64) decimal.Decimal {
	return decimal.NewFromInt(v)
}

func TestProjectExpensePaceRequiresSpend(t *testing.T) {
	planned := expensePaceMoney(25000)

	assert.Nil(t, ProjectExpensePace(expensePaceMoney(0), &planned, expensePaceAsOf(20)))
	assert.Nil(t, ProjectExpensePace(expensePaceMoney(-100), &planned, expensePaceAsOf(20)))
}

func TestProjectExpensePaceWaitsForMinimumDays(t *testing.T) {
	planned := expensePaceMoney(25000)

	assert.Nil(t, ProjectExpensePace(expensePaceMoney(10000), &planned, expensePaceAsOf(MinElapsedDaysForCalendarPace-1)))
	assert.NotNil(t, ProjectExpensePace(expensePaceMoney(100), &planned, expensePaceAsOf(MinElapsedDaysForCalendarPace)))
}

func TestProjectExpensePaceProjectsMonthEnd(t *testing.T) {
	planned := expensePaceMoney(25000)

	pace := ProjectExpensePace(expensePaceMoney(10000), &planned, expensePaceAsOf(10))
	require.NotNil(t, pace)
	assert.Equal(t, "2026-09", pace.Period)
	assert.Equal(t, "30000", pace.ProjectedExpenses.String())
}

func TestProjectExpensePaceOverBudget(t *testing.T) {
	planned := expensePaceMoney(25000)

	pace := ProjectExpensePace(expensePaceMoney(10000), &planned, expensePaceAsOf(10))
	require.NotNil(t, pace)
	require.NotNil(t, pace.OverBudget)
	assert.Equal(t, "5000", pace.OverBudget.String())
}

func TestProjectExpensePaceUnderBudget(t *testing.T) {
	planned := expensePaceMoney(25000)

	pace := ProjectExpensePace(expensePaceMoney(5000), &planned, expensePaceAsOf(10))
	require.NotNil(t, pace)
	assert.Equal(t, "15000", pace.ProjectedExpenses.String())
	assert.Nil(t, pace.OverBudget)
}

func TestProjectExpensePaceWithoutPlan(t *testing.T) {
	pace := ProjectExpensePace(expensePaceMoney(10000), nil, expensePaceAsOf(10))
	require.NotNil(t, pace)
	assert.Equal(t, "30000", pace.ProjectedExpenses.String())
	assert.Nil(t, pace.OverBudget)

	zero := expensePaceMoney(0)
	pace = ProjectExpensePace(expensePaceMoney(10000), &zero, expensePaceAsOf(10))
	require.NotNil(t, pace)
	assert.Nil(t, pace.OverBudget)
}
