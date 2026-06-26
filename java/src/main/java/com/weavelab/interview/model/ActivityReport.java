package com.weavelab.interview.model;

import com.fasterxml.jackson.annotation.JsonProperty;
import java.util.Map;

public class ActivityReport {

    @JsonProperty("user_id")
    private String userId;

    @JsonProperty("total_actions")
    private int totalActions;

    @JsonProperty("by_action")
    private Map<String, Integer> byAction;

    @JsonProperty("by_resource")
    private Map<String, Integer> byResource;

    public ActivityReport() {}

    public ActivityReport(String userId, int totalActions,
                          Map<String, Integer> byAction, Map<String, Integer> byResource) {
        this.userId = userId;
        this.totalActions = totalActions;
        this.byAction = byAction;
        this.byResource = byResource;
    }

    public String getUserId() { return userId; }
    public void setUserId(String userId) { this.userId = userId; }

    public int getTotalActions() { return totalActions; }
    public void setTotalActions(int totalActions) { this.totalActions = totalActions; }

    public Map<String, Integer> getByAction() { return byAction; }
    public void setByAction(Map<String, Integer> byAction) { this.byAction = byAction; }

    public Map<String, Integer> getByResource() { return byResource; }
    public void setByResource(Map<String, Integer> byResource) { this.byResource = byResource; }
}
