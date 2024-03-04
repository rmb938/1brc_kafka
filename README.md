# 1brc_kafka
A version of the 1 billion row challange but using Kafka as the data store and Golang as the programming language.

All data is generated via the official 1brc scripts found here https://github.com/gunnarmorling/1brc/tree/main?tab=readme-ov-file#running-the-challenge

# Requirements

* Go 1.21+
* Java to compile and generate 1brc data
* A Kafka compatible server 
  * [Apache Kafka](https://kafka.apache.org/)
  * [Redpanda](https://redpanda.com/)
  * [Warpstream](https://www.warpstream.com/)

# Local Development

A docker-compose file is available to develop the code locally

1. Clone 1brc `git clone https://github.com/gunnarmorling/1brc.git`
1. Follow instructions here to generate measruements
1. Run `docker compose up` and wait for kminion container to start
1. Run `go run cmd/producer/main.go`

# Setup & Testing

While these tests can be ran on any machine that can run Kafka and a compiled Golang binary there are provided setups in the `terraform` directory.

If using a Kafka implementation that needs disk it is recommended to use instance types that have local NVMe disks instead of attached storage over the network.

For all instance types it is also recommended to use virtual machines that have at least 10Gbps of network bandwidth.

The terraform will upload the generated data into a bucket and the producing instances will download the file before starting the test.

Prometheus, kminion and node_exporter are also deployed to monitor the Kafka cluster as well as the producing and consuming applications.

# TODO

- [X] Producing
- [ ] Consuming
- [ ] Computing Average
