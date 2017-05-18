package lgctx_test

import (
	"context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"

	"cred-alert/lgctx"
)

var _ = Describe("Lager Context", func() {
	It("can store loggers inside contexts", func() {
		l := lagertest.NewTestLogger("lgctx")
		ctx := lgctx.NewContext(context.Background(), l)

		logger := lgctx.FromContext(ctx)
		logger.Info("from-a-context")

		Expect(l.LogMessages()).To(HaveLen(1))
	})

	It("can add a session to the logger in the context", func() {
		l := lagertest.NewTestLogger("lgctx")
		ctx := lgctx.NewContext(context.Background(), l)

		logger := lgctx.WithSession(ctx, "new-session", lager.Data{
			"bespoke-data": "",
		})
		logger.Info("from-a-context")

		Expect(l).To(gbytes.Say("new-session"))
		Expect(l).To(gbytes.Say("bespoke-data"))
	})

	It("can add data to the logger in the context", func() {
		l := lagertest.NewTestLogger("lgctx")
		ctx := lgctx.NewContext(context.Background(), l)

		logger := lgctx.WithData(ctx, lager.Data{
			"bespoke-data": "",
		})
		logger.Info("from-a-context")

		Expect(l).To(gbytes.Say("bespoke-data"))
	})

	It("will be fine if there is no logger in the context", func() {
		logger := lgctx.FromContext(context.Background())
		logger.Info("from-a-context")
	})
})
