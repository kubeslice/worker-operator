package workerslicegwrecycler

import "strings"

func getNewDeploymentName(gwID string) string {
	l := strings.Split(gwID, "-")
	depInstance := l[len(l)-1]

	newDepInstance := "1"

	if depInstance == "0" {
		newDepInstance = "1"
	} else {
		newDepInstance = "0"
	}

	l[len(l)-1] = newDepInstance

	return strings.Join(l, "-")
}

func getRequestString(req Request) string {
	switch req {
	case REQ_none:
		return "none"
	case REQ_create_new_deployment:
		return "create_new_deployment"
	case REQ_update_routing_table:
		return "update_routing_table"
	case REQ_delete_old_gw_deployment:
		return "delete_old_gw_deployment"
	default:
		return ""
	}
}

func getRequestIndex(req string) Request {
	switch req {
	case "none":
		return REQ_none
	case "create_new_deployment":
		return REQ_create_new_deployment
	case "update_routing_table":
		return REQ_update_routing_table
	case "delete_old_gw_deployment":
		return REQ_delete_old_gw_deployment
	default:
		return REQ_invalid
	}
}

func getResponseString(resp Response) string {
	switch resp {
	case RESP_none:
		return "none"
	case RESP_new_deployment_created:
		return "new_deployment_created"
	case RESP_routing_table_updated:
		return "routing_table_updated"
	case RESP_old_deployment_deleted:
		return "old_gw_deployment_deleted"
	default:
		return ""
	}
}

func getResponseIndex(resp string) Response {
	switch resp {
	case "none":
		return RESP_none
	case "new_deployment_created":
		return RESP_new_deployment_created
	case "routing_table_updated":
		return RESP_routing_table_updated
	case "old_gw_deployment_deleted":
		return RESP_old_deployment_deleted
	default:
		return RESP_invalid
	}
}
