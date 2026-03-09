package domain

import "testing"

func TestOrderTypeEnums(t *testing.T) {
	tests := []struct {
		name   string
		check  func() bool
	}{
		{
			name: "OrderTypeBuy defined",
			check: func() bool {
				return OrderTypeBuy == "buy"
			},
		},
		{
			name: "OrderTypeSell defined",
			check: func() bool {
				return OrderTypeSell == "sell"
			},
		},
		{
			name: "BUY in SupportedOrderTypes",
			check: func() bool {
				return SupportedOrderTypes[OrderTypeBuy]
			},
		},
		{
			name: "SELL in SupportedOrderTypes",
			check: func() bool {
				return SupportedOrderTypes[OrderTypeSell]
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !tt.check() {
				t.Fail()
			}
		})
	}
}

func TestOrderStatusEnums(t *testing.T) {
	tests := []struct {
		name   string
		check  func() bool
	}{
		{
			name: "StatusPending defined",
			check: func() bool {
				return StatusPending == "pending"
			},
		},
		{
			name: "StatusMatched defined",
			check: func() bool {
				return StatusMatched == "matched"
			},
		},
		{
			name: "StatusSettled defined",
			check: func() bool {
				return StatusSettled == "settled"
			},
		},
		{
			name: "StatusCancelled defined",
			check: func() bool {
				return StatusCancelled == "cancelled"
			},
		},
		{
			name: "Pending in SupportedOrderStatuses",
			check: func() bool {
				return SupportedOrderStatuses[StatusPending]
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !tt.check() {
				t.Fail()
			}
		})
	}
}

func TestValidStatusTransitions(t *testing.T) {
	tests := []struct {
		name      string
		current   OrderStatus
		next      OrderStatus
		expected  bool
	}{
		{
			name:     "pending -> matched allowed",
			current:  StatusPending,
			next:     StatusMatched,
			expected: true,
		},
		{
			name:     "pending -> cancelled allowed",
			current:  StatusPending,
			next:     StatusCancelled,
			expected: true,
		},
		{
			name:     "pending -> settled not allowed",
			current:  StatusPending,
			next:     StatusSettled,
			expected: false,
		},
		{
			name:     "matched -> settled allowed",
			current:  StatusMatched,
			next:     StatusSettled,
			expected: true,
		},
		{
			name:     "settled -> matched not allowed",
			current:  StatusSettled,
			next:     StatusMatched,
			expected: false,
		},
		{
			name:     "cancelled -> pending not allowed",
			current:  StatusCancelled,
			next:     StatusPending,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if IsValidTransition(tt.current, tt.next) != tt.expected {
				t.Errorf("IsValidTransition(%q, %q) = %v, want %v",
					tt.current, tt.next, !tt.expected, tt.expected)
			}
		})
	}
}

func TestTimeInForceEnums(t *testing.T) {
	tests := []struct {
		name   string
		check  func() bool
	}{
		{
			name: "TimeInForceIOC defined",
			check: func() bool {
				return TimeInForceIOC == "ioc"
			},
		},
		{
			name: "TimeInForceGTC defined",
			check: func() bool {
				return TimeInForceGTC == "gtc"
			},
		},
		{
			name: "IOC in SupportedTimeInForce",
			check: func() bool {
				return SupportedTimeInForce[TimeInForceIOC]
			},
		},
		{
			name: "GTC in SupportedTimeInForce",
			check: func() bool {
				return SupportedTimeInForce[TimeInForceGTC]
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !tt.check() {
				t.Fail()
			}
		})
	}
}
