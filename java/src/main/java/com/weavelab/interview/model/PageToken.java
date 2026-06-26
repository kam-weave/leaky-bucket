package com.weavelab.interview.model;

import java.time.Instant;

public class PageToken {

    private Instant createdAt;
    private String id;

    public PageToken() {}

    public PageToken(Instant createdAt, String id) {
        this.createdAt = createdAt;
        this.id = id;
    }

    public Instant getCreatedAt() { return createdAt; }
    public void setCreatedAt(Instant createdAt) { this.createdAt = createdAt; }

    public String getId() { return id; }
    public void setId(String id) { this.id = id; }
}
