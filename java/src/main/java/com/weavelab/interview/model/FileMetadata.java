package com.weavelab.interview.model;

import com.fasterxml.jackson.annotation.JsonProperty;
import java.time.Instant;

public class FileMetadata {

    private String id;
    private String filename;
    private long size;

    @JsonProperty("content_type")
    private String contentType;

    @JsonProperty("created_at")
    private Instant createdAt;

    public FileMetadata() {}

    public FileMetadata(String id, String filename, long size, String contentType, Instant createdAt) {
        this.id = id;
        this.filename = filename;
        this.size = size;
        this.contentType = contentType;
        this.createdAt = createdAt;
    }

    public String getId() { return id; }
    public void setId(String id) { this.id = id; }

    public String getFilename() { return filename; }
    public void setFilename(String filename) { this.filename = filename; }

    public long getSize() { return size; }
    public void setSize(long size) { this.size = size; }

    public String getContentType() { return contentType; }
    public void setContentType(String contentType) { this.contentType = contentType; }

    public Instant getCreatedAt() { return createdAt; }
    public void setCreatedAt(Instant createdAt) { this.createdAt = createdAt; }
}
