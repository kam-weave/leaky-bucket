package com.weavelab.interview.config;

import org.springframework.beans.factory.annotation.Value;
import org.springframework.context.annotation.Configuration;

import java.nio.file.Path;

@Configuration
public class AppConfig {

    @Value("${app.data-dir}")
    private String dataDir;

    @Value("${app.max-upload-size}")
    private long maxUploadSize;

    public String getDataDir() {
        return dataDir;
    }

    public String getFilesDir() {
        return Path.of(dataDir, "files").toString();
    }

    public long getMaxUploadSize() {
        return maxUploadSize;
    }
}
