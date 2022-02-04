package manager

import (
	"context"
	"encoding/base64"
	"os"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	log "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"bitbucket.org/realtimeai/kubeslice-operator/internal/hub/controllers"
	"bitbucket.org/realtimeai/kubeslice-operator/internal/logger"
	spokev1alpha1 "bitbucket.org/realtimeai/mesh-hub/apis/mesh/v1alpha1"
)

const (
	ns = "hub-avesha-tenant-cisco"
)

var scheme = runtime.NewScheme()

func init() {
	log.SetLogger(logger.NewLogger())
	clientgoscheme.AddToScheme(scheme)
	utilruntime.Must(spokev1alpha1.AddToScheme(scheme))
}

func Start(ctx context.Context) {

	ca := "LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUVMRENDQXBTZ0F3SUJBZ0lRS3pEQXovWVZXcCtZampEcFMrVDN2REFOQmdrcWhraUc5dzBCQVFzRkFEQXYKTVMwd0t3WURWUVFERXlReE16VmtaR1kzTXkwd05qZzBMVFF6WVRJdE9UUXpaQzFoWmpVMFpqRTVOMkZpTVdFdwpJQmNOTWpJd01URTRNRGsxTnpFd1doZ1BNakExTWpBeE1URXhNRFUzTVRCYU1DOHhMVEFyQmdOVkJBTVRKREV6Ck5XUmtaamN6TFRBMk9EUXRORE5oTWkwNU5ETmtMV0ZtTlRSbU1UazNZV0l4WVRDQ0FhSXdEUVlKS29aSWh2Y04KQVFFQkJRQURnZ0dQQURDQ0FZb0NnZ0dCQU5JQkY2SlVQMlljSXNEMmxjK3hBUmFySTdSSmNMYmZaQWR1UUJ4MQo4MCtJRzlYZk04K25kSCtwNTJwSmJqZXJWTzMycWZKUnJWYzNtSVNjQ0RERjVveklMZXVPQWp6NkZTVTEzT3c3ClJPYlJWS1E0R1RMS3c3aWxlSkpNMThSdkxhUzJ1YjJsZ2VzcnB1bndmYjMxR2tOTzFUNUlFTFdIcDY0amFPMW8KLzFlMzFiSVFOTnMvTGQ0WFZUeHlXdkt6aEhSNjJyWGFTMzhOY0wzVFRTd1E3VzJ4U2cyVWZqOUh2eU53dE8rRApOblNqUGpNZVBwVytWTWJjSFRLZUcySkdFUlJTL0JHUXpYNnF1a1BQSGdlYmNiMC9CeWlPVlZlclNvcEhCc09vClRhVTNZMG90VVk3Mjg1TEZQTitjajcwMWhTWGVYaTc3WTR0enlGL01xeENXNENvQmNnZjc4KzNRZjNKSHZjTnMKL1MvekJOaUV0bXBPWmRvclJQcDhFYkh2cnYzK1Y4Qjg0ZDAxL2M1dG9pZmh2S0ttQ1FBRXdrOVZRaUQ1Z1BQMQpVQ3hhZDJzSHdraHBTRWUyQVU4Z2RUZmd5cFJ2ZEtaVXVrZUNvWncyS0hTRDAxR2NqY3l2UlJncm82THFYTHBYClRWeFJaSkhSYnB2d3B4QW5pWkw0VWxrUVhRSURBUUFCbzBJd1FEQU9CZ05WSFE4QkFmOEVCQU1DQWdRd0R3WUQKVlIwVEFRSC9CQVV3QXdFQi96QWRCZ05WSFE0RUZnUVVlQUhiVi9lWEdDZlR1cEhLMFcxVG1yWlYzdnN3RFFZSgpLb1pJaHZjTkFRRUxCUUFEZ2dHQkFGK3lzSE9zU3U1MGxiU1N5OUZNam9QWTlwMkVNRlNMa2I1RmRXY3NXdnlRCkFuRVE1NVZyTVNCeU52c0pFRlYrYmROVmFGclExSjZnZGtBZUhMSHpPdHVyRmJZUE1QOVpJKzEvdk1PaTlTTHIKUFVzZWVXbDVwUVdEbUNRVE9QMXNRSExjR0hPc1RTSUg4cHFIclpNTEpQOGpGZzZmUDZRbFMyZEs5bFZWRFhJLwpSdjgrNnFkUGRaMzBuOG9Qd25sWDBpKy9xbzRMVlFEZmpxamx2U1IzQTJWUXo0aGtvbkovKzdzaUdhTkZrMlBmCjNxektGK252UnJqa3R5WDd3cnN0cndZVS9IWFFBZTFkOHNhSGlQWnJuQ2hmWURhWmkzNUd3bElCbzdsUmMzeUQKTTd2bHVtRWJMcDRHbE9pbTU4QkNZV0JvcXZaWHNvN1NkWHhrYUtTZ0NMT1ZKWDBQckxlTEs4RjBUcllUblhOZgp4dzF1Z21jZ3dvQ0U2UHppYk5sZTAyajVCV2pQQnAvN1p1cFJYVGR4QTlGZjlFRVRvcHFhODVJUjBKQitvUkZ3CmdjM09aY05HZVg2V2dPYSthK0NSK01RR2RmRkZOM3lzZVNKR3IxRys0M1kreEtrNEVYdkdzQnk2Q2V6SVYyUFUKOVhMZ0pPYXZ2a1J6T3d1MkZrbHZ0QT09Ci0tLS0tRU5EIENFUlRJRklDQVRFLS0tLS0K"
	caBundle, _ := base64.RawStdEncoding.DecodeString(ca)

	config := &rest.Config{
		Host:        "https://35.190.142.81",
		BearerToken: "eyJhbGciOiJSUzI1NiIsImtpZCI6IjJkWmFoM19ST3g2QVdHN1lzNVhmQWJMTXNfeWo3UkxBMHhIVThrMmpRdDQifQ.eyJpc3MiOiJrdWJlcm5ldGVzL3NlcnZpY2VhY2NvdW50Iiwia3ViZXJuZXRlcy5pby9zZXJ2aWNlYWNjb3VudC9uYW1lc3BhY2UiOiJodWItYXZlc2hhLXRlbmFudC1jaXNjbyIsImt1YmVybmV0ZXMuaW8vc2VydmljZWFjY291bnQvc2VjcmV0Lm5hbWUiOiJrdWJlLXNsaWNlLXNwb2tlLWNsdXN0ZXItOC1qZC10b2tlbi1iNnRmOCIsImt1YmVybmV0ZXMuaW8vc2VydmljZWFjY291bnQvc2VydmljZS1hY2NvdW50Lm5hbWUiOiJrdWJlLXNsaWNlLXNwb2tlLWNsdXN0ZXItOC1qZCIsImt1YmVybmV0ZXMuaW8vc2VydmljZWFjY291bnQvc2VydmljZS1hY2NvdW50LnVpZCI6IjdhYjBmOTc1LWZmMmMtNGFjMC05NTk0LWFmYTExOTIyNDNlMyIsInN1YiI6InN5c3RlbTpzZXJ2aWNlYWNjb3VudDpodWItYXZlc2hhLXRlbmFudC1jaXNjbzprdWJlLXNsaWNlLXNwb2tlLWNsdXN0ZXItOC1qZCJ9.fjazB-KVQnuXQGDuug16hR8H3IFcRAZHxgZhdFtPXx7odlBjpGnrvpZbhw9Tbt7HsXnsS_WpJAcwPj4WjNhSHeOKu1PDuCjrkvVyOsLcPrDDmB-Jd3-K4xru2YlNgxunJjEDuuWaNKKlTjfLZ755CRbrDMD2VdQ15o4Dart4OakQiEgjYkooxyp6zUDKk5I6SnHXml4l9HTty0Ti40KjYCzDgnAZ6aOWYYN5rbDmUSc3fqjAaZ_NhGFXhi-5NYbqjyoOwiDzUzRLoIl3B3MZ8x_RhG2X1sVODmZExp-Ka2MbGHW9tEPE0ReENnH4CbY7ztBeTecRVFAK8F0U3rBGBQ",
		TLSClientConfig: rest.TLSClientConfig{
			CAData: caBundle,
		},
	}

	var log = log.Log.WithName("hub")

	mgr, err := manager.New(config, manager.Options{
		Namespace:          ns,
		Scheme:             scheme,
		MetricsBindAddress: "0", // disable metrics for now
	})
	if err != nil {
		log.Error(err, "could not create manager")
		os.Exit(1)
	}

	err = builder.
		ControllerManagedBy(mgr).
		For(&spokev1alpha1.Slice{}).
		Complete(&controllers.SliceReconciler{})
	if err != nil {
		log.Error(err, "could not create controller")
		os.Exit(1)
	}

	if err := mgr.Start(ctx); err != nil {
		log.Error(err, "could not start manager")
		os.Exit(1)
	}

}
