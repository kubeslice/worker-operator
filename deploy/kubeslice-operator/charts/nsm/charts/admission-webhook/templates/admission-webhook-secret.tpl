apiVersion: v1
kind: Secret
metadata:
  name: nsm-admission-webhook-certs
  namespace: {{ .Release.Namespace }}
type: Opaque
data:
  tls.key: LS0tLS1CRUdJTiBSU0EgUFJJVkFURSBLRVktLS0tLQpNSUlFcEFJQkFBS0NBUUVBcXhPVjJMYm1taTdzZVhjTUMvTUd1b2dsZlBmNzZuU2hMS0JsRk5lcnlvUVZQWmdPCk9CV3VMek1HUHRxeDZTRXo2YXJtMXg0b1JUZkFOMWI4ZjV3R3BXdXRYSGtBdEtMYngyTWRKd2Y1VUNGTmY2UHoKZW1KbmVaL3QxdVpxSGRoQWZrZWxCdFF5dFRyU3p3MWh0bkJnOTNnQnpoVllvMndSRElMekt3TytMaG1iSlFaVApRaDV4VUxqN29iY0Vlb0RmdzRsNHdYQzlCczlZZTgxMW5iWWRwbTROQXlSeTYrMEVoWVh5aitNWmUxSXdoVkxkCitrUlU5c0pNdlhDWm14d3E4bHNva3pRbk9iaVdwQlJqQTVYRjVtaEJFd3orL2VhNmdjTzVzQ1VibGhldUFVbjMKTWpKNlF3OEdEWGxlUFZuSklMUllsbHRZQU1EaW02aG9QeTBDcndJREFRQUJBb0lCQUhIYUMreDQ5SWtCMTNDUwpzSnEzTnZBbXNVUTB5UnRrV09zWko0d3laK3JUOGtyV2lnZjdMYnZOcWtka1JlaVBwenZIOSs2TDdHTDhVbGpCCjlESjh4Tk9NRUlpdElySVVmRTE2Z2FrN0hrbWNrRFgxQjVHWU1hTDRzMUZFY0xUQitWSFJIbHVvRnNNVGpiNHIKK3E1dXBhbXIzUStvbHgvVFNKbGFBTGpNdWVGMUk5NTIzekQyU1lMVG82OVYwZGRYUnNnMjhXUHFhOHZ5UlJMSApxcmtwSUxLeHQ3M2tMM28zN1lxUXZOeW82T1RUWlhUbHVPUVVFTmU3ekZWWHFuVlJaRVhtUmV0OGk5SzFhZCtFCmV1RlZucWJySVZHNjRwRkFhb2k0QWQ5cWpOU2YzZm10aE9Gd0s4ZVVkU0RPcGdnakdFTC9jOENtSUdFd3BqUUEKUTIwN0JYRUNnWUVBdzIzemlXcGd3VzZvQ3lnKzdHZ1BmaHZXWXJVcnpJcTVpK3hhNnpWVVBYY0s2aytCekF5Ywpkclo2NXFQMkJpQ0ZyQnlUUWZjNWZVWFF2amZFUTNxQ01KdHh5RVhpSU91TjJCYjZWQUFTSmIxNjlCeGVEVVgyClMzdnBXUkl5b1I3QU8wblFza01FM2k1Ym52clkweDg3aFBBZ0Y1RGtEN1NmTDVkYkp5YUY4QmtDZ1lFQTRCbGkKUTVoNGF2WldzNk1Jdy9MbnVQQXBIL2xoMlR1NXFpNmhnWG11Uk1RL2dzRE9tYzNxTzAyNVN3alB1WUxjMkRsMwo0UWJ3b3JDL0FnS3FRUURDbGZUNEloMmNMRGNYcUxlR1ZteHo1clZsNkJRcU5XeEFac3lCSTZwSjBORmRVR3J6CkdaeUk1RG1QWTRzcVVmR1hWY1M3M25XZHI4Z0psejBDc21ZbFFnY0NnWUVBa2Z4RWZGWVd5T2djWjVrOHgrUkUKRG5SRkJaOUloSmJzVy9YSFJRU2xWUFRrRm53bC9ZTStMZi9LZHhmcjVFL1BDdTZkb2gxSHVLaTZjaDIrWXBuVgpQdklmWVBlekg5eFdMU0dkQmJxMzA3RmpjNDd0UXdVTUl2OEJKU1JPNWNUTzNIc2JoczVCaUtjZ2tmWFltbjB1ClBQUVRSUWRiRmRCYlNYWExCY2ZsTGFFQ2dZRUFyVkUwZkU3cG91QU9RalJ2VFEwS1JqQUh2bURqV2wwa3hRZjMKaE9tVTdENVRXRTdCK3BZVTkvU3V2K2Q2c0dFVGFHOVoxY0hHVGkwZ0xPL2V1Uk5iYXhyZzVaRzgvVDFHb1FmLwpiOHZFLzhOL296UWxTTmdHSHZzL1RWUWdiczNkdTVwYmxZMUpHaW1pU2p5UmFIck9ybGpQYThmUFF1b1U4TkVRCnl1VFJIL1VDZ1lCVCtsZnlxVVRLSURWNnNLSUhGU3ZWdXFpVllrU0hadzRJZk9EVVNGTWRXdVFTY2FaNFZVWW8KQUpFc2IxVWF6NWNVYlFDU3h1ZDU3NlFqaXptUElzMktKbzFtWmYwT2lOWldVT0R2SnltanVXUjIyZFVkWmoyMwpjWHVRYk1RWUkvMWk0R3ZMVHJJeHNiSnlGUjZ1NThMd3c1S0xRYitzM2F4YWhhSk5aSFMybVE9PQotLS0tLUVORCBSU0EgUFJJVkFURSBLRVktLS0tLQo=
  tls.crt: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUNrakNDQWppZ0F3SUJBZ0lSQUkxa25pOXVmOFZ2RllxNGVJQkpvbUV3Q2dZSUtvWkl6ajBFQXdJd0d6RVoKTUJjR0ExVUVBeE1RYlhrdGMyVnNabk5wWjI1bFpDMWpZVEFlRncweU1qQXlNakV3TWpRek1URmFGdzB5TWpBMQpNakl3TWpRek1URmFNQUF3Z2dFaU1BMEdDU3FHU0liM0RRRUJBUVVBQTRJQkR3QXdnZ0VLQW9JQkFRQ3JFNVhZCnR1YWFMdXg1ZHd3TDh3YTZpQ1Y4OS92cWRLRXNvR1VVMTZ2S2hCVTltQTQ0RmE0dk13WSsyckhwSVRQcHF1YlgKSGloRk44QTNWdngvbkFhbGE2MWNlUUMwb3R2SFl4MG5CL2xRSVUxL28vTjZZbWQ1biszVzVtb2QyRUIrUjZVRwoxREsxT3RMUERXRzJjR0QzZUFIT0ZWaWpiQkVNZ3ZNckE3NHVHWnNsQmxOQ0huRlF1UHVodHdSNmdOL0RpWGpCCmNMMEd6MWg3elhXZHRoMm1iZzBESkhMcjdRU0ZoZktQNHhsN1VqQ0ZVdDM2UkZUMndreTljSm1iSENyeVd5aVQKTkNjNXVKYWtGR01EbGNYbWFFRVREUDc5NXJxQnc3bXdKUnVXRjY0QlNmY3lNbnBERHdZTmVWNDlXY2tndEZpVwpXMWdBd09LYnFHZy9MUUt2QWdNQkFBR2pnYXd3Z2Frd0RnWURWUjBQQVFIL0JBUURBZ1dnTUF3R0ExVWRFd0VCCi93UUNNQUF3SHdZRFZSMGpCQmd3Rm9BVWhqbWZXZVhiUTlYZzJ2djU3TlE3N2g0UEZYRXdhQVlEVlIwUkFRSC8KQkY0d1hJSXFibk50TFdGa2JXbHpjMmx2YmkxM1pXSm9iMjlyTFhOMll5NXJkV0psYzJ4cFkyVXRjM2x6ZEdWdApnaTV1YzIwdFlXUnRhWE56YVc5dUxYZGxZbWh2YjJzdGMzWmpMbXQxWW1WemJHbGpaUzF6ZVhOMFpXMHVjM1pqCk1Bb0dDQ3FHU000OUJBTUNBMGdBTUVVQ0lBRTcwWllzQXdNV0dhUHp1TlVwK09hMngwR25UWnVJNzdEc00zUDkKY2t6S0FpRUFqMzhqV09jcitsaHlQUGt5dDBIaUhuMUpWUXFac1F1Z1NxWlpPSVh1UFh3PQotLS0tLUVORCBDRVJUSUZJQ0FURS0tLS0tCg==
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nsm-admission-webhook
  namespace: {{ .Release.Namespace }}
  labels:
    app: nsm-admission-webhook
spec:
  replicas: 1
  selector:
    matchLabels:
      app: nsm-admission-webhook
  template:
    metadata:
      labels:
        app: nsm-admission-webhook
    spec:
      containers:
        - name: nsm-admission-webhook
          image: {{ .Values.registry }}/{{ .Values.org }}/admission-webhook:{{ .Values.tag }}
          imagePullPolicy: {{ .Values.pullPolicy }}
          env:
            - name: REPO
              value: "{{ .Values.org }}"
            - name: TAG
              value: "{{ .Values.tag }}"
            - name: NSM_NAMESPACE
              value: "{{ .Values.clientNamespace }}"
            - name: TRACER_ENABLED
              value: {{ .Values.global.JaegerTracing | default false | quote }}
            - name: JAEGER_AGENT_HOST
              value: jaeger.{{ .Release.Namespace }}
            - name: JAEGER_AGENT_PORT
              value: "6831"
          volumeMounts:
            - name: webhook-certs
              mountPath: /etc/webhook/certs
              readOnly: true
          livenessProbe:
            httpGet:
              path: /liveness
              port: 5555
            initialDelaySeconds: 10
            periodSeconds: 10
            timeoutSeconds: 3
          readinessProbe:
            httpGet:
              path: /readiness
              port: 5555
            initialDelaySeconds: 10
            periodSeconds: 10
            timeoutSeconds: 3
      volumes:
        - name: webhook-certs
          secret:
            secretName: nsm-admission-webhook-certs
---
apiVersion: v1
kind: Service
metadata:
  name: nsm-admission-webhook-svc
  namespace: {{ .Release.Namespace }}
  labels:
    app: nsm-admission-webhook
spec:
  ports:
    - port: 443
      targetPort: 443
  selector:
    app: nsm-admission-webhook
---
apiVersion: admissionregistration.k8s.io/v1beta1
kind: MutatingWebhookConfiguration
metadata:
  name: nsm-admission-webhook-cfg
  namespace: {{ .Release.Namespace }}
  labels:
    app: nsm-admission-webhook
webhooks:
  - name: admission-webhook.networkservicemesh.io
    clientConfig:
      service:
        name: nsm-admission-webhook-svc
        namespace: {{ .Release.Namespace }}
        path: "/mutate"
      caBundle: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUJkakNDQVIyZ0F3SUJBZ0lSQU1RMENRK3Q2RjNSMkhZMnFEVVFlTnd3Q2dZSUtvWkl6ajBFQXdJd0d6RVoKTUJjR0ExVUVBeE1RYlhrdGMyVnNabk5wWjI1bFpDMWpZVEFlRncweU1qQXlNVFl4TVRReU1qRmFGdzB5TWpBMQpNVGN4TVRReU1qRmFNQnN4R1RBWEJnTlZCQU1URUcxNUxYTmxiR1p6YVdkdVpXUXRZMkV3V1RBVEJnY3Foa2pPClBRSUJCZ2dxaGtqT1BRTUJCd05DQUFTWVE4aHE0YWFxbVJUK203dTV3NE96eFpSWjFOM1ZrRGhYOC9XZXBRZ0wKL1ZYMzI3UjBYNEFFdXlMR3JHYmUyQURmSndGa0hEa05EVW1wY3JadFBxZ2pvMEl3UURBT0JnTlZIUThCQWY4RQpCQU1DQXFRd0R3WURWUjBUQVFIL0JBVXdBd0VCL3pBZEJnTlZIUTRFRmdRVWhqbWZXZVhiUTlYZzJ2djU3TlE3CjdoNFBGWEV3Q2dZSUtvWkl6ajBFQXdJRFJ3QXdSQUlnQjJWTENZZ3ZKQ3ZTWG83RGVQaGl4VXQ0S3BtOXlUdmoKUXhIYTc5UjF6Z29DSUZyMkVYV3J4V0xxSWJYRDl2YU9qUys1RzZaZ1J4d05UM2FaaFQ5R1FRbFQKLS0tLS1FTkQgQ0VSVElGSUNBVEUtLS0tLQo=
    rules:
      - operations: ["CREATE"]
        apiGroups: ["apps", "extensions", ""]
        apiVersions: ["v1", "v1beta1"]
        resources: ["deployments", "services", "pods"]
