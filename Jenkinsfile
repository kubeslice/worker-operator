@Library('jenkins-library@opensource-helm-pipeline') _
dockerImagePipeline1(
  script: this,
  service: 'aveshasystems/worker-operator',
  dockerfile: 'Dockerfile',
  buildContext: '.',
  buildArguments: [PLATFORM:"amd64"]
)
