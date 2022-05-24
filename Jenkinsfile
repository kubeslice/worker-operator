@Library('jenkins-library@unit-opensource') _
dockerbuildtestPipeline(
  script: this,
  service: 'aveshasystems/worker-operator',
  dockerfile: 'Dockerfile',
  buildContext: '.',
  buildArguments: [PLATFORM:"amd64"]
)
