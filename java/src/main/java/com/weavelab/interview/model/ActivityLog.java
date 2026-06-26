package com.weavelab.interview.model;

import com.fasterxml.jackson.annotation.JsonProperty;
import java.time.Instant;

public class ActivityLog {

    private Long id;

    @JsonProperty("user_id")
    private String userId;

    private String action;

    @JsonProperty("resource_type")
    private String resourceType;

    @JsonProperty("resource_id")
    private String resourceId;

    @JsonProperty("created_at")
    private Instant createdAt;

    public ActivityLog() {}

    public ActivityLog(Long id, String userId, String action, String resourceType,
                       String resourceId, Instant createdAt) {
        this.id = id;
        this.userId = userId;
        this.action = action;
        this.resourceType = resourceType;
        this.resourceId = resourceId;
        this.createdAt = createdAt;
    }

    public Long getId() { return id; }
    public void setId(Long id) { this.id = id; }

    public String getUserId() { return userId; }
    public void setUserId(String userId) { this.userId = userId; }

    public String getAction() { return action; }
    public void setAction(String action) { this.action = action; }

    public String getResourceType() { return resourceType; }
    public void setResourceType(String resourceType) { this.resourceType = resourceType; }

    public String getResourceId() { return resourceId; }
    public void setResourceId(String resourceId) { this.resourceId = resourceId; }

    public Instant getCreatedAt() { return createdAt; }
    public void setCreatedAt(Instant createdAt) { this.createdAt = createdAt; }
}
