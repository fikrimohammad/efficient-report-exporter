package repository

type MySQL interface {
	ReportMySQL
}

type MQ interface {
	ReportMQ
}

type Redis interface {
	ReportRedis
}

type S3 interface {
	ReportS3
}
