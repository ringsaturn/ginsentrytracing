package ginsentrytracing

import (
	"fmt"
	"net/http"

	"github.com/getsentry/sentry-go"
	"github.com/gin-gonic/gin"
)

const (
	_TRANSACTION_KEY      = "_sentry_gin_span"
	SENTRY_TRACE_HEADER   = "sentry-trace" // https://develop.sentry.dev/sdk/performance/#header-sentry-trace
	SENTRY_BAGGAGE_HEADER = "baggage"      // https://develop.sentry.dev/sdk/performance/dynamic-sampling-context/#baggage
)

// Convert HTTP Status to [Sentry status], rewrite from Sentry Python SDK's [tracing.py]
//
// [Sentry status]: https://develop.sentry.dev/sdk/event-payloads/properties/status/
// [tracing.py]: https://github.com/getsentry/sentry-python/blob/1.12.0/sentry_sdk/tracing.py#L436-L467
func FromHTTPStatusToSentryStatus(code int) sentry.SpanStatus {
	if code < http.StatusBadRequest {
		return sentry.SpanStatusOK
	}
	if http.StatusBadRequest <= code && code < http.StatusInternalServerError {
		switch code {
		case http.StatusForbidden:
			return sentry.SpanStatusPermissionDenied
		case http.StatusNotFound:
			return sentry.SpanStatusNotFound
		case http.StatusTooManyRequests:
			return sentry.SpanStatusResourceExhausted
		case http.StatusRequestEntityTooLarge:
			return sentry.SpanStatusFailedPrecondition
		case http.StatusUnauthorized:
			return sentry.SpanStatusUnauthenticated
		case http.StatusConflict:
			return sentry.SpanStatusAlreadyExists
		default:
			return sentry.SpanStatusInvalidArgument
		}
	}
	if http.StatusInternalServerError <= code && code < 600 {
		switch code {
		case http.StatusGatewayTimeout:
			return sentry.SpanStatusDeadlineExceeded
		case http.StatusNotImplemented:
			return sentry.SpanStatusUnimplemented
		case http.StatusServiceUnavailable:
			return sentry.SpanStatusUnavailable
		default:
			return sentry.SpanStatusInternalError
		}
	}
	return sentry.SpanStatusUnknown
}

type Option struct {
	GetTraceIDFromRequest func(*gin.Context) string
	GetBaggageFromRequest func(*gin.Context) string
}

func NewDefaultOption() *Option {
	return &Option{
		GetTraceIDFromRequest: func(ctx *gin.Context) string { return ctx.GetHeader(SENTRY_TRACE_HEADER) },
		GetBaggageFromRequest: func(ctx *gin.Context) string { return ctx.GetHeader(SENTRY_BAGGAGE_HEADER) },
	}
}

type Options = func(*Option)

func AttachSpan(opts ...Options) gin.HandlerFunc {
	defaultOpt := NewDefaultOption()
	for _, optFunc := range opts {
		optFunc(defaultOpt)
	}

	return func(ctx *gin.Context) {
		name := fmt.Sprintf("%v_%v", ctx.Request.Method, ctx.FullPath())
		trace := defaultOpt.GetTraceIDFromRequest(ctx)
		baggage := defaultOpt.GetBaggageFromRequest(ctx)
		transaction := sentry.StartTransaction(ctx, name, sentry.ContinueFromHeaders(trace, baggage))
		ctx.Set(_TRANSACTION_KEY, transaction)
		ctx.Writer.Header().Set(SENTRY_TRACE_HEADER, transaction.ToSentryTrace())
		ctx.Writer.Header().Set(SENTRY_BAGGAGE_HEADER, transaction.ToBaggage())
		ctx.Next()
		transaction.Status = FromHTTPStatusToSentryStatus(ctx.Writer.Status())
		transaction.Finish()
	}
}

func StartSpanFromGinContext(ctx *gin.Context, op string) *sentry.Span {
	span, ok := ctx.Value(_TRANSACTION_KEY).(*sentry.Span)
	if ok && span != nil {
		return span.StartChild(op)
	}
	return sentry.StartSpan(ctx, op)
}
