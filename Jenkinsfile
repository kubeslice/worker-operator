@Library('jenkins-library@master') _
dockerImagePipeline(
  script: this,
  service: 'kubeslice/operator',
  dockerfile: 'Dockerfile',
  buildContext: '.',
  buildArguments: [PLATFORM:"amd64"]
)