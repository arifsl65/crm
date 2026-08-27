# =============================================================================
# Accountant CRM - MNS (Message Service) Topics
# =============================================================================
# MNS service activated - resources enabled
# =============================================================================

resource "alicloud_mns_topic" "topics" {
  for_each = toset(var.mns_topics)

  name                 = "${local.name_prefix}-${each.value}"
  maximum_message_size = 65536 # 64KB max message size
  logging_enabled      = true
}

# =============================================================================
# Dead Letter Queue
# =============================================================================

resource "alicloud_mns_queue" "dlq" {
  name                     = "${local.name_prefix}-dlq"
  delay_seconds            = 0
  maximum_message_size     = 65536
  message_retention_period = 604800 # 7 days (max allowed)
  visibility_timeout       = 300    # 5 minutes
  polling_wait_seconds     = 20
}

# =============================================================================
# Topic Subscriptions (Push to DLQ for failed messages)
# =============================================================================

resource "alicloud_mns_topic_subscription" "dlq_subscription" {
  for_each = toset(var.mns_topics)

  topic_name = alicloud_mns_topic.topics[each.value].name
  name       = "${each.value}-dlq-sub"
  # FIXED: Endpoint must be full ARN format for queue subscriptions
  endpoint              = "acs:mns:${var.region}:${data.alicloud_account.current.id}:queues/${alicloud_mns_queue.dlq.name}"
  notify_strategy       = "EXPONENTIAL_DECAY_RETRY"
  notify_content_format = "JSON"
}

# =============================================================================
# Processing Queues (One per topic for consumers)
# =============================================================================

resource "alicloud_mns_queue" "processing" {
  for_each = toset(var.mns_topics)

  name                     = "${local.name_prefix}-${each.value}-queue"
  delay_seconds            = 0
  maximum_message_size     = 65536
  message_retention_period = 345600 # 4 days
  visibility_timeout       = 120    # 2 minutes
  polling_wait_seconds     = 20
}

# =============================================================================
# Topic to Queue Subscriptions
# =============================================================================

resource "alicloud_mns_topic_subscription" "queue_subscription" {
  for_each = toset(var.mns_topics)

  topic_name = alicloud_mns_topic.topics[each.value].name
  name       = "${each.value}-queue-sub"
  # FIXED: Endpoint must be full ARN format for queue subscriptions
  endpoint              = "acs:mns:${var.region}:${data.alicloud_account.current.id}:queues/${alicloud_mns_queue.processing[each.value].name}"
  notify_strategy       = "EXPONENTIAL_DECAY_RETRY"
  notify_content_format = "JSON"
}

# =============================================================================
# Outputs
# =============================================================================

output "mns_topics" {
  description = "MNS topic details"
  value = {
    for k, v in alicloud_mns_topic.topics : k => {
      name = v.name
      arn  = "acs:mns:${var.region}:${data.alicloud_account.current.id}:topics/${v.name}"
    }
  }
}

output "mns_queues" {
  description = "MNS queue details"
  value = {
    for k, v in alicloud_mns_queue.processing : k => {
      name = v.name
      arn  = "acs:mns:${var.region}:${data.alicloud_account.current.id}:queues/${v.name}"
    }
  }
}

output "mns_dlq" {
  description = "MNS dead letter queue"
  value = {
    name = alicloud_mns_queue.dlq.name
    arn  = "acs:mns:${var.region}:${data.alicloud_account.current.id}:queues/${alicloud_mns_queue.dlq.name}"
  }
}
