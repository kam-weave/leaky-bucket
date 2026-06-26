package com.weavelab.interview.config;

import org.springframework.beans.factory.annotation.Value;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.core.io.ClassPathResource;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.jdbc.datasource.init.ResourceDatabasePopulator;

import javax.sql.DataSource;
import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;

@Configuration
public class DataSourceConfig {

    @Value("${app.data-dir}")
    private String dataDir;

    @Bean
    public JdbcTemplate jdbcTemplate(DataSource dataSource) throws IOException {
        Path dataDirPath = Path.of(dataDir);
        Files.createDirectories(dataDirPath);
        Files.createDirectories(dataDirPath.resolve("files"));

        ResourceDatabasePopulator populator = new ResourceDatabasePopulator();
        populator.addScript(new ClassPathResource("schema.sql"));
        populator.execute(dataSource);

        return new JdbcTemplate(dataSource);
    }

    public String getFilesDir() {
        return Path.of(dataDir, "files").toString();
    }
}
