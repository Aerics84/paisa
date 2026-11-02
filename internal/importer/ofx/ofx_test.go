package ofx

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const sampleOFX = `OFXHEADER:100
DATA:OFXSGML
VERSION:102
SECURITY:NONE
ENCODING:USASCII
CHARSET:1252
COMPRESSION:NONE
OLDFILEUID:NONE
NEWFILEUID:NONE

<OFX>
<SIGNONMSGSRSV1>
<SONRS>
<STATUS>
<CODE>0
<SEVERITY>INFO
</STATUS>
<DTSERVER>20261010120000[0:GMT]
<LANGUAGE>ENG
</SONRS>
</SIGNONMSGSRSV1>
<BANKMSGSRSV1>
<STMTTRNRS>
<TRNUID>1001
<STATUS>
<CODE>0
<SEVERITY>INFO
</STATUS>
<STMTRS>
<CURDEF>USD
<BANKACCTFROM>
<BANKID>123456789
<ACCTID>987654321
<ACCTTYPE>CHECKING
</BANKACCTFROM>
<BANKTRANLIST>
<DTSTART>20260901000000
<DTEND>20260930235959
<STMTTRN>
<TRNTYPE>DEBIT
<DTPOSTED>20260915120000
<TRNAMT>-85.50
<FITID>TXN001
<NAME>WHOLE FOODS MKT
<MEMO>GROCERIES
</STMTTRN>
<STMTTRN>
<TRNTYPE>CREDIT
<DTPOSTED>20260920083000
<TRNAMT>3500.00
<FITID>TXN002
<NAME>ACME CORP PAYROLL
<MEMO>SALARY
</STMTTRN>
</BANKTRANLIST>
<LEDGERBAL>
<BALAMT>8414.50
<DTASOF>20260930235959
</LEDGERBAL>
</STMTRS>
</STMTTRNRS>
</BANKMSGSRSV1>
</OFX>`

const sampleXML = `<?xml version="1.0" encoding="UTF-8"?>
<?OFX OFXHEADER="200" VERSION="211" SECURITY="NONE" OLDFILEUID="NONE" NEWFILEUID="NONE"?>
<OFX>
  <BANKMSGSRSV1>
    <STMTTRNRS>
      <STMTRS>
        <CURDEF>EUR</CURDEF>
        <BANKACCTFROM>
          <ACCTID>12345678</ACCTID>
          <ACCTTYPE>CHECKING</ACCTTYPE>
        </BANKACCTFROM>
        <BANKTRANLIST>
          <STMTTRN>
            <TRNTYPE>CHECK</TRNTYPE>
            <DTPOSTED>20261005</DTPOSTED>
            <TRNAMT>-120.00</TRNAMT>
            <FITID>XML001</FITID>
            <MEMO>RENT PAYMENT</MEMO>
          </STMTTRN>
          <STMTTRN>
            <TRNTYPE>CREDIT</TRNTYPE>
            <DTPOSTED>20261006143000</DTPOSTED>
            <TRNAMT>250.75</TRNAMT>
            <FITID>XML002</FITID>
            <NAME>REFUND</NAME>
          </STMTTRN>
        </BANKTRANLIST>
      </STMTRS>
    </STMTTRNRS>
  </BANKMSGSRSV1>
</OFX>`

func TestParseSGML(t *testing.T) {
	stmt, err := Parse(sampleOFX)
	require.NoError(t, err)
	assert.Equal(t, "USD", stmt.Currency)
	assert.Equal(t, "987654321", stmt.AccountID)
	require.Len(t, stmt.Transactions, 2)

	tx1 := stmt.Transactions[0]
	assert.Equal(t, "TXN001", tx1.FITID)
	assert.Equal(t, "DEBIT", tx1.Type)
	assert.Equal(t, "WHOLE FOODS MKT", tx1.Payee)
	assert.Equal(t, "GROCERIES", tx1.Memo)
	assert.Equal(t, -85.50, tx1.Amount)
	assert.Equal(t, 85.50, tx1.AbsAmount())
	assert.True(t, tx1.IsDebit())
	assert.Equal(t, time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC), tx1.Date)

	tx2 := stmt.Transactions[1]
	assert.Equal(t, "TXN002", tx2.FITID)
	assert.Equal(t, "CREDIT", tx2.Type)
	assert.Equal(t, "ACME CORP PAYROLL", tx2.Payee)
	assert.Equal(t, "SALARY", tx2.Memo)
	assert.Equal(t, 3500.00, tx2.Amount)
	assert.Equal(t, 3500.00, tx2.AbsAmount())
	assert.False(t, tx2.IsDebit())
	assert.Equal(t, time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC), tx2.Date)
}

func TestParseXML(t *testing.T) {
	stmt, err := Parse(sampleXML)
	require.NoError(t, err)
	assert.Equal(t, "EUR", stmt.Currency)
	assert.Equal(t, "12345678", stmt.AccountID)
	require.Len(t, stmt.Transactions, 2)

	tx1 := stmt.Transactions[0]
	assert.Equal(t, "XML001", tx1.FITID)
	assert.Equal(t, "CHECK", tx1.Type)
	assert.Equal(t, "RENT PAYMENT", tx1.Payee)
	assert.Equal(t, "RENT PAYMENT", tx1.Memo)
	assert.Equal(t, -120.00, tx1.Amount)
	assert.Equal(t, 120.00, tx1.AbsAmount())
	assert.True(t, tx1.IsDebit())
	assert.Equal(t, time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC), tx1.Date)

	tx2 := stmt.Transactions[1]
	assert.Equal(t, "XML002", tx2.FITID)
	assert.Equal(t, "CREDIT", tx2.Type)
	assert.Equal(t, "REFUND", tx2.Payee)
	assert.Empty(t, tx2.Memo)
	assert.Equal(t, 250.75, tx2.Amount)
	assert.False(t, tx2.IsDebit())
	assert.Equal(t, time.Date(2026, 10, 6, 0, 0, 0, 0, time.UTC), tx2.Date)
}

func TestParseMetadata(t *testing.T) {
	content := `<OFX>
<CURDEF>GBP
<BANKACCTFROM>
<ACCTID>555666
</BANKACCTFROM>
<STMTTRN>
<TRNTYPE>DEBIT
<DTPOSTED>20260101
<TRNAMT>-1.00
<FITID>M1
<NAME>SHOP
</STMTTRN>
</OFX>`

	stmt, err := Parse(content)
	require.NoError(t, err)
	assert.Equal(t, "GBP", stmt.Currency)
	assert.Equal(t, "555666", stmt.AccountID)
	require.Len(t, stmt.Transactions, 1)
	assert.Equal(t, "M1", stmt.Transactions[0].FITID)
}

func TestTransactionAbsAmountAndIsDebit(t *testing.T) {
	tests := []struct {
		name        string
		amount      float64
		txnType     string
		absAmount   float64
		wantIsDebit bool
	}{
		{name: "negative debit", amount: -10.25, txnType: "DEBIT", absAmount: 10.25, wantIsDebit: true},
		{name: "positive credit", amount: 20, txnType: "CREDIT", absAmount: 20, wantIsDebit: false},
		{name: "negative check", amount: -5, txnType: "CHECK", absAmount: 5, wantIsDebit: true},
		{name: "positive check", amount: 5, txnType: "CHECK", absAmount: 5, wantIsDebit: false},
		{name: "zero debit", amount: 0, txnType: "DEBIT", absAmount: 0, wantIsDebit: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tx := Transaction{Amount: tt.amount, Type: tt.txnType}
			assert.Equal(t, tt.absAmount, tx.AbsAmount())
			assert.Equal(t, tt.wantIsDebit, tx.IsDebit())
		})
	}
}

func TestParseRejectsEmptyContent(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{name: "empty", content: ""},
		{name: "no transactions", content: "<OFX>\n<CURDEF>USD\n</OFX>"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stmt, err := Parse(tt.content)
			require.Error(t, err)
			assert.Nil(t, stmt)
		})
	}
}

func TestParseSkipsInvalidTransaction(t *testing.T) {
	content := `<OFX>
<CURDEF>USD
<STMTTRN>
<TRNTYPE>DEBIT
<DTPOSTED>20260101
<FITID>BAD1
<NAME>NO AMOUNT
</STMTTRN>
<STMTTRN>
<TRNTYPE>CREDIT
<DTPOSTED>20260102
<TRNAMT>15.00
<FITID>GOOD1
<NAME>OK
</STMTTRN>
</OFX>`

	stmt, err := Parse(content)
	require.NoError(t, err)
	require.Len(t, stmt.Transactions, 1)
	assert.Equal(t, "GOOD1", stmt.Transactions[0].FITID)

	invalidOnly := `<OFX>
<STMTTRN>
<TRNTYPE>DEBIT
<DTPOSTED>20260101
<FITID>BAD1
<NAME>NO AMOUNT
</STMTTRN>
</OFX>`

	stmt, err = Parse(invalidOnly)
	require.Error(t, err)
	assert.Nil(t, stmt)
}
