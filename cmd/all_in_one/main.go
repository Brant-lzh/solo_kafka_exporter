package main

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"flag"
	"github.com/Shopify/sarama"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	plog "github.com/prometheus/common/promlog"
	plogflag "github.com/prometheus/common/promlog/flag"
	"github.com/prometheus/common/version"
	"github.com/rcrowley/go-metrics"
	"gopkg.in/alecthomas/kingpin.v2"
	"io/ioutil"
	"k8s.io/klog/v2"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

var (
	opts        = kafkaOpts{}
	topicFilter string
	groupFilter string
)

func init() {
	metrics.UseNilMetrics = true
	prometheus.MustRegister(version.NewCollector("kafka_exporter"))
}

func main() {
	var (
		listenAddress    = toFlagString("web.listen-address", "Address to listen on for web interface and telemetry.", ":9308")
		metricsPath      = toFlagString("web.telemetry-path", "Path under which to expose self program metrics.", "/metrics")
		kafkaMetricsPath = toFlagString("kafka.telemetry-path", "Path under which to expose kafka metrics.", "/kafka/metrics")
		tF               = toFlagString("topic.filter", "Regex that determines which topics to collect.", ".*")
		gF               = toFlagString("group.filter", "Regex that determines which consumer groups to collect.", ".*")
		logSarama        = toFlagBool("log.enable-sarama", "Turn on Sarama logging, default is false.", false, "false")
	)
	topicFilter = *tF
	groupFilter = *gF

	toFlagStringsVar("kafka.server", "Address (host:port) of Kafka server.", "kafka:9092", &opts.uri)
	toFlagBoolVar("sasl.enabled", "Connect using SASL/PLAIN, default is false.", false, "false", &opts.useSASL)
	toFlagBoolVar("sasl.handshake", "Only set this to false if using a non-Kafka SASL proxy, default is true.", true, "true", &opts.useSASLHandshake)
	toFlagStringVar("sasl.username", "SASL user name.", "", &opts.saslUsername)
	toFlagStringVar("sasl.password", "SASL user password.", "", &opts.saslPassword)
	toFlagStringVar("sasl.mechanism", "The SASL SCRAM SHA algorithm sha256 or sha512 or gssapi as mechanism", "", &opts.saslMechanism)
	toFlagStringVar("sasl.service-name", "Service name when using kerberos Auth", "", &opts.serviceName)
	toFlagStringVar("sasl.kerberos-config-path", "Kerberos config path", "", &opts.kerberosConfigPath)
	toFlagStringVar("sasl.realm", "Kerberos realm", "", &opts.realm)
	toFlagStringVar("sasl.kerberos-auth-type", "Kerberos auth type. Either 'keytabAuth' or 'userAuth'", "", &opts.kerberosAuthType)
	toFlagStringVar("sasl.keytab-path", "Kerberos keytab file path", "", &opts.keyTabPath)
	toFlagBoolVar("sasl.disable-PA-FX-FAST", "Configure the Kerberos client to not use PA_FX_FAST, default is false.", false, "false", &opts.saslDisablePAFXFast)
	toFlagBoolVar("tls.enabled", "Connect to Kafka using TLS, default is false.", false, "false", &opts.useTLS)
	toFlagStringVar("tls.server-name", "Used to verify the hostname on the returned certificates unless tls.insecure-skip-tls-verify is given. The kafka server's name should be given.", "", &opts.tlsServerName)
	toFlagStringVar("tls.ca-file", "The optional certificate authority file for Kafka TLS client authentication.", "", &opts.tlsCAFile)
	toFlagStringVar("tls.cert-file", "The optional certificate file for Kafka client authentication.", "", &opts.tlsCertFile)
	toFlagStringVar("tls.key-file", "The optional key file for Kafka client authentication.", "", &opts.tlsKeyFile)
	toFlagBoolVar("server.tls.enabled", "Enable TLS for web server, default is false.", false, "false", &opts.serverUseTLS)
	toFlagBoolVar("server.tls.mutual-auth-enabled", "Enable TLS client mutual authentication, default is false.", false, "false", &opts.serverMutualAuthEnabled)
	toFlagStringVar("server.tls.ca-file", "The certificate authority file for the web server.", "", &opts.serverTlsCAFile)
	toFlagStringVar("server.tls.cert-file", "The certificate file for the web server.", "", &opts.serverTlsCertFile)
	toFlagStringVar("server.tls.key-file", "The key file for the web server.", "", &opts.serverTlsKeyFile)
	toFlagBoolVar("tls.insecure-skip-tls-verify", "If true, the server's certificate will not be checked for validity. This will make your HTTPS connections insecure. Default is false", false, "false", &opts.tlsInsecureSkipTLSVerify)
	toFlagStringVar("kafka.version", "Kafka broker version", sarama.V2_0_0_0.String(), &opts.kafkaVersion)
	toFlagBoolVar("use.consumelag.zookeeper", "if you need to use a group from zookeeper, default is false", false, "false", &opts.useZooKeeperLag)
	toFlagStringsVar("zookeeper.server", "Address (hosts) of zookeeper server.", "localhost:2181", &opts.uriZookeeper)
	toFlagStringVar("kafka.labels", "Kafka cluster name", "", &opts.labels)
	toFlagStringVar("refresh.metadata", "Metadata refresh interval", "30s", &opts.metadataRefreshInterval)
	toFlagBoolVar("offset.show-all", "Whether show the offset/lag for all consumer group, otherwise, only show connected consumer groups, default is true", true, "true", &opts.offsetShowAll)
	toFlagBoolVar("concurrent.enable", "If true, all scrapes will trigger kafka operations otherwise, they will share results. WARN: This should be disabled on large clusters. Default is false", false, "false", &opts.allowConcurrent)
	toFlagIntVar("topic.workers", "Number of topic workers", 100, "100", &opts.topicWorkers)
	toFlagBoolVar("kafka.allow-auto-topic-creation", "If true, the broker may auto-create topics that we requested which do not already exist, default is false.", false, "false", &opts.allowAutoTopicCreation)
	toFlagIntVar("verbosity", "Verbosity log level", 0, "0", &opts.verbosityLogLevel)

	plConfig := plog.Config{}
	plogflag.AddFlags(kingpin.CommandLine, &plConfig)
	kingpin.Version(version.Print("kafka_exporter"))
	kingpin.HelpFlag.Short('h')
	kingpin.Parse()

	labels := make(map[string]string)

	// Protect against empty labels
	if opts.labels != "" {
		for _, label := range strings.Split(opts.labels, ",") {
			splitted := strings.Split(label, "=")
			if len(splitted) >= 2 {
				labels[splitted[0]] = splitted[1]
			}
		}
	}

	setup(*listenAddress, *metricsPath, *kafkaMetricsPath, *logSarama, labels)
}

func setup(
	listenAddress string,
	metricsPath string,
	kafkaMetricsPath string,
	logSarama bool,
	labels map[string]string,
) {
	klog.InitFlags(flag.CommandLine)
	if err := flag.Set("logtostderr", "true"); err != nil {
		klog.Errorf("Error on setting logtostderr to true: %v", err)
	}
	err := flag.Set("v", strconv.Itoa(opts.verbosityLogLevel))
	if err != nil {
		klog.Errorf("Error on setting v to %v: %v", strconv.Itoa(opts.verbosityLogLevel), err)
	}
	defer klog.Flush()

	klog.V(INFO).Infoln("Starting kafka_exporter", version.Info())
	klog.V(DEBUG).Infoln("Build context", version.BuildContext())

	clusterBrokers = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "brokers"),
		"Number of Brokers in the Kafka Cluster.",
		nil, labels,
	)
	clusterBrokerInfo = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "broker_info"),
		"Information about the Kafka Broker.",
		[]string{"id", "address"}, labels,
	)
	topicPartitions = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "topic", "partitions"),
		"Number of partitions for this Topic",
		[]string{"topic"}, labels,
	)
	topicCurrentOffset = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "topic", "partition_current_offset"),
		"Current Offset of a Broker at Topic/Partition",
		[]string{"topic", "partition"}, labels,
	)
	topicOldestOffset = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "topic", "partition_oldest_offset"),
		"Oldest Offset of a Broker at Topic/Partition",
		[]string{"topic", "partition"}, labels,
	)

	topicPartitionLeader = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "topic", "partition_leader"),
		"Leader Broker ID of this Topic/Partition",
		[]string{"topic", "partition"}, labels,
	)

	topicPartitionReplicas = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "topic", "partition_replicas"),
		"Number of Replicas for this Topic/Partition",
		[]string{"topic", "partition"}, labels,
	)

	topicPartitionInSyncReplicas = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "topic", "partition_in_sync_replica"),
		"Number of In-Sync Replicas for this Topic/Partition",
		[]string{"topic", "partition"}, labels,
	)

	topicPartitionUsesPreferredReplica = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "topic", "partition_leader_is_preferred"),
		"1 if Topic/Partition is using the Preferred Broker",
		[]string{"topic", "partition"}, labels,
	)

	topicUnderReplicatedPartition = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "topic", "partition_under_replicated_partition"),
		"1 if Topic/Partition is under Replicated",
		[]string{"topic", "partition"}, labels,
	)

	consumergroupCurrentOffset = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "consumergroup", "current_offset"),
		"Current Offset of a ConsumerGroup at Topic/Partition",
		[]string{"consumergroup", "topic", "partition"}, labels,
	)

	consumergroupCurrentOffsetSum = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "consumergroup", "current_offset_sum"),
		"Current Offset of a ConsumerGroup at Topic for all partitions",
		[]string{"consumergroup", "topic"}, labels,
	)

	consumergroupLag = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "consumergroup", "lag"),
		"Current Approximate Lag of a ConsumerGroup at Topic/Partition",
		[]string{"consumergroup", "topic", "partition"}, labels,
	)

	consumergroupLagZookeeper = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "consumergroupzookeeper", "lag_zookeeper"),
		"Current Approximate Lag(zookeeper) of a ConsumerGroup at Topic/Partition",
		[]string{"consumergroup", "topic", "partition"}, nil,
	)

	consumergroupLagSum = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "consumergroup", "lag_sum"),
		"Current Approximate Lag of a ConsumerGroup at Topic for all partitions",
		[]string{"consumergroup", "topic"}, labels,
	)

	consumergroupMembers = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "consumergroup", "members"),
		"Amount of members in a consumer group",
		[]string{"consumergroup"}, labels,
	)

	if logSarama {
		sarama.Logger = log.New(os.Stdout, "[sarama] ", log.LstdFlags)
	}

	http.Handle(metricsPath, promhttp.Handler())

	http.HandleFunc(kafkaMetricsPath, kafkaHandler)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		_, err := w.Write([]byte(`<html>
	        <head><title>Kafka Exporter</title></head>
	        <body>
	        <h1>Kafka Exporter</h1>
	        <p><a href='` + metricsPath + `'>Metrics</a></p>
	        </body>
	        </html>`))
		if err != nil {
			klog.Error("Error handle / request", err)
		}
	})

	http.HandleFunc("/info", func(w http.ResponseWriter, r *http.Request) {
		cache.mutex.RLock()
		defer cache.mutex.RUnlock()
		bytes, _ := json.Marshal(cache.mp)
		_, err := w.Write(bytes)
		if err != nil {
			klog.Error("Error handle /info request", err)
		}
		return
	})

	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		// need more specific sarama check
		_, err := w.Write([]byte("ok"))
		if err != nil {
			klog.Error("Error handle /healthz request", err)
		}
	})

	//start web
	if opts.serverUseTLS {
		klog.V(INFO).Infoln("Listening on HTTPS", listenAddress)

		_, err := CanReadCertAndKey(opts.serverTlsCertFile, opts.serverTlsKeyFile)
		if err != nil {
			klog.Error("error reading server cert and key")
		}

		clientAuthType := tls.NoClientCert
		if opts.serverMutualAuthEnabled {
			clientAuthType = tls.RequireAndVerifyClientCert
		}

		certPool := x509.NewCertPool()
		if opts.serverTlsCAFile != "" {
			if caCert, err := ioutil.ReadFile(opts.serverTlsCAFile); err == nil {
				certPool.AppendCertsFromPEM(caCert)
			} else {
				klog.Error("error reading server ca")
			}
		}

		tlsConfig := &tls.Config{
			ClientCAs:                certPool,
			ClientAuth:               clientAuthType,
			MinVersion:               tls.VersionTLS12,
			CurvePreferences:         []tls.CurveID{tls.CurveP521, tls.CurveP384, tls.CurveP256},
			PreferServerCipherSuites: true,
			CipherSuites: []uint16{
				tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
				tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256,
				tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_RSA_WITH_AES_256_CBC_SHA,
				tls.TLS_RSA_WITH_AES_128_CBC_SHA256,
			},
		}
		server := &http.Server{
			Addr:      listenAddress,
			TLSConfig: tlsConfig,
		}
		klog.Fatal(server.ListenAndServeTLS(opts.serverTlsCertFile, opts.serverTlsKeyFile))
	} else {
		klog.V(INFO).Infoln("Listening on HTTP", listenAddress)
		klog.Fatal(http.ListenAndServe(listenAddress, nil))
	}
}

func kafkaHandler(w http.ResponseWriter, r *http.Request) {
	request, err := banding(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	var (
		mr    *multiRegistry
		found bool
	)
	if mr, found = cache.Get(request.String()); !found {
		op := opts
		op.uri = request.Addresses
		op.kafkaVersion = request.Version
		//new exporter
		exporter, err := NewExporter(op, topicFilter, groupFilter)
		if err != nil {
			klog.Fatalln(err)
			w.WriteHeader(http.StatusBadRequest)
			unescape, _ := url.QueryUnescape(r.URL.RawQuery)
			_, _ = w.Write([]byte(err.Error() + unescape))
			return
		}
		mr = NewMultiRegistry(exporter)
		//defer exporter.client.Close()

		//set
		cache.Set(request.String(), mr)
	} else {
		mr.SetUpdateTime(time.Now())
	}

	promhttp.HandlerFor(mr.registry, promhttp.HandlerOpts{}).ServeHTTP(w, r)
	return
}
