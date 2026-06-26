package com.weavelab.interview.repository;

import com.weavelab.interview.config.AppConfig;
import com.weavelab.interview.model.FileMetadata;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.jdbc.core.RowMapper;
import org.springframework.stereotype.Repository;

import java.io.IOException;
import java.io.InputStream;
import java.io.OutputStream;
import java.nio.file.Files;
import java.nio.file.Path;
import java.sql.Timestamp;
import java.time.Instant;
import java.util.List;
import java.util.Optional;

@Repository
public class FileRepository {

    private final JdbcTemplate jdbc;
    private final AppConfig appConfig;

    private static final RowMapper<FileMetadata> ROW_MAPPER = (rs, rowNum) -> new FileMetadata(
            rs.getString("id"),
            rs.getString("filename"),
            rs.getLong("size"),
            rs.getString("content_type"),
            rs.getTimestamp("created_at").toInstant()
    );

    public FileRepository(JdbcTemplate jdbc, AppConfig appConfig) {
        this.jdbc = jdbc;
        this.appConfig = appConfig;
    }

    public FileMetadata create(String id, String filename, String contentType, InputStream content) throws IOException {
        Instant now = Instant.now();
        Path filePath = Path.of(appConfig.getFilesDir(), id);

        long size;
        try (OutputStream out = Files.newOutputStream(filePath)) {
            size = content.transferTo(out);
        } catch (IOException e) {
            Files.deleteIfExists(filePath);
            throw e;
        }

        try {
            jdbc.update("""
                INSERT INTO files (id, filename, size, content_type, created_at)
                VALUES (?, ?, ?, ?, ?)
                """, id, filename, size, contentType, Timestamp.from(now));
        } catch (Exception e) {
            Files.deleteIfExists(filePath);
            throw e;
        }

        return new FileMetadata(id, filename, size, contentType, now);
    }

    public Optional<FileMetadata> findById(String id) {
        List<FileMetadata> results = jdbc.query("""
            SELECT id, filename, size, content_type, created_at
            FROM files WHERE id = ?
            """, ROW_MAPPER, id);
        return results.isEmpty() ? Optional.empty() : Optional.of(results.get(0));
    }

    public InputStream openFile(String id) throws IOException {
        Path filePath = Path.of(appConfig.getFilesDir(), id);
        return Files.newInputStream(filePath);
    }

    public List<FileMetadata> list() {
        return jdbc.query("""
            SELECT id, filename, size, content_type, created_at
            FROM files
            ORDER BY created_at DESC
            """, ROW_MAPPER);
    }
}
