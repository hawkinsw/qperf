module github.com/hawkinsw/qperf

go 1.16

//replace github.com/lucas-clemente/quic-go => ../quic-go

replace github.com/hawkinsw/qperf/tlsconfig => ./tlsconfig
replace github.com/hawkinsw/qperf/logging => ./logging
replace github.com/hawkinsw/qperf/utils => ./utils
replace github.com/hawkinsw/qperf/resultwriter => ./resultwriter

//require github.com/lucas-clemente/quic-go v0.0.0
require github.com/lucas-clemente/quic-go v0.21.1

require github.com/hawkinsw/qperf/tlsconfig v0.0.0
require github.com/hawkinsw/qperf/logging v0.0.0
require github.com/hawkinsw/qperf/utils v0.0.0
require github.com/hawkinsw/qperf/resultwriter v0.0.0
