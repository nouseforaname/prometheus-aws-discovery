package awsdiscovery

import (
	"errors"
	"regexp"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/daspawnw/prometheus-aws-discovery/pkg/discovery"
	log "github.com/sirupsen/logrus"
)

type DiscoveryClientAWS struct {
	TagPrefix string
	Tag       string
}

var ec2Client ec2iface.EC2API

func (d DiscoveryClientAWS) SetEC2Client(client ec2iface.EC2API) {
	ec2Client = client
}
func (d DiscoveryClientAWS) GetInstances() ([]discovery.Instance, error) {

	ec2Input := ec2.DescribeInstancesInput{
		Filters: d.filter(),
	}
	if ec2Client == nil {
		awsSession := session.New()
		awsConfig := &aws.Config{}
		ec2Client = ec2.New(awsSession, awsConfig)

	}
	output, err := ec2Client.DescribeInstances(&ec2Input)
	if err != nil {
		log.Error("Failed to load ec2 instances")
		return nil, err
	}

	instances := []discovery.Instance{}
	endpointCount := 0
	for _, res := range output.Reservations {
		for _, instance := range res.Instances {

			log.Debugf("Extract metric endpoint(s) from instance with ip %s", *instance.PrivateIpAddress)
			metricEndpoints := extractMetricEndpoints(instance.Tags, d.TagPrefix)
			endpointCount += len(metricEndpoints)
			log.Debugf("Instance with ip %s has %d metric endpoint(s)", *instance.PrivateIpAddress, len(metricEndpoints))

			d := discovery.Instance{
				InstanceType: *instance.InstanceType,
				PrivateIP:    *instance.PrivateIpAddress,
				Tags:         cleanupTagList(instance.Tags, d.TagPrefix),
				Metrics:      metricEndpoints,
			}

			instances = append(instances, d)
		}
	}
	log.Infof("Discovered %d instance(s) with %d endpoint(s)", len(instances), endpointCount)
	return instances, nil
}

func (d DiscoveryClientAWS) filter() []*ec2.Filter {
	filters := []*ec2.Filter{
		{
			Name:   aws.String("tag-key"),
			Values: []*string{aws.String(d.TagPrefix + "*")},
		},
		{
			Name:   aws.String("instance-state-name"),
			Values: []*string{aws.String("running")},
		},
	}

	return filters
}

func cleanupTagList(tags []*ec2.Tag, prefix string) map[string]string {
	tagMap := make(map[string]string)

	for _, tag := range tags {
		// remove prom/scrape annotation and aws based tags from tag list
		if !matchKeyPattern(*tag.Key, prefix) && !strings.Contains(*tag.Key, ":") {
			tagMap[*tag.Key] = *tag.Value
		}
	}

	return tagMap
}

func extractMetricEndpoints(tags []*ec2.Tag, prefix string) []discovery.InstanceMetrics {
	metrics := []discovery.InstanceMetrics{}

	for _, tag := range tags {
		if matchKeyPattern(*tag.Key, prefix) {
			parsedMetric, err := parseMetricEndpoint(*tag.Key, *tag.Value, prefix)
			if err != nil {
				log.Error("Failed to parse Tag, skip metric", err)
				continue
			}

			metrics = append(metrics, *parsedMetric)
		}
	}

	return metrics
}

func parseMetricEndpoint(key string, value string, prefix string) (*discovery.InstanceMetrics, error) {
	r, _ := regexp.Compile(prefix + ":(.*?)(/.*)")
	parsedMetric := r.FindStringSubmatch(key)

	if len(parsedMetric) == 3 {
		//TO BE FAIR, NONE OF THIS IS A GOOD IDEA. THIS IS AT BEST A DIRTY AND FUGLY QUICK FIX. THIS SHOULD BE REFACTORED TO DO SOMETHING REASONABLE
		var scheme = "http"
		parsedString := strings.Split(parsedMetric[1], ":")
		if len(parsedString) == 2 {
			scheme = parsedString[1]
		}

		parsedPort, err := strconv.ParseInt(parsedString[0], 10, 64)
		if err != nil {
			return nil, err
		}

		return &discovery.InstanceMetrics{
			Name:   value,
			Path:   parsedMetric[2],
			Port:   parsedPort,
			Scheme: scheme,
		}, nil
	}

	return nil, errors.New("Failed to match regex pattern")
}

func matchKeyPattern(key string, prefix string) bool {
	r, _ := regexp.Compile(prefix + ":(.*?)(/.*)")

	return r.MatchString(key)
}
