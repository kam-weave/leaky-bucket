package com.weavelab.interview.repository;

import com.weavelab.interview.model.Contact;
import com.weavelab.interview.model.PageToken;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.jdbc.core.RowMapper;
import org.springframework.stereotype.Repository;

import java.sql.Timestamp;
import java.time.Instant;
import java.util.List;
import java.util.Optional;

@Repository
public class ContactRepository {

    private final JdbcTemplate jdbc;

    private static final RowMapper<Contact> ROW_MAPPER = (rs, rowNum) -> new Contact(
            rs.getString("id"),
            rs.getString("first_name"),
            rs.getString("last_name"),
            rs.getString("email"),
            rs.getString("phone"),
            rs.getString("company"),
            rs.getTimestamp("created_at").toInstant(),
            rs.getTimestamp("updated_at").toInstant()
    );

    public ContactRepository(JdbcTemplate jdbc) {
        this.jdbc = jdbc;
    }

    public List<Contact> list(int limit, PageToken cursor) {
        if (cursor == null) {
            return jdbc.query("""
                SELECT id, first_name, last_name, email, phone, company, created_at, updated_at
                FROM contacts
                ORDER BY created_at DESC, id DESC
                LIMIT ?
                """, ROW_MAPPER, limit);
        }
        return jdbc.query("""
            SELECT id, first_name, last_name, email, phone, company, created_at, updated_at
            FROM contacts
            WHERE (created_at, id) < (?, ?)
            ORDER BY created_at DESC, id DESC
            LIMIT ?
            """, ROW_MAPPER, Timestamp.from(cursor.getCreatedAt()), cursor.getId(), limit);
    }

    public Optional<Contact> findById(String id) {
        List<Contact> results = jdbc.query("""
            SELECT id, first_name, last_name, email, phone, company, created_at, updated_at
            FROM contacts WHERE id = ?
            """, ROW_MAPPER, id);
        return results.isEmpty() ? Optional.empty() : Optional.of(results.get(0));
    }

    public void create(Contact contact) {
        Instant now = Instant.now();
        contact.setCreatedAt(now);
        contact.setUpdatedAt(now);

        jdbc.update("""
            INSERT INTO contacts (id, first_name, last_name, email, phone, company, created_at, updated_at)
            VALUES (?, ?, ?, ?, ?, ?, ?, ?)
            """,
            contact.getId(), contact.getFirstName(), contact.getLastName(),
            contact.getEmail(), contact.getPhone(), contact.getCompany(),
            Timestamp.from(contact.getCreatedAt()), Timestamp.from(contact.getUpdatedAt()));
    }

    public boolean update(Contact contact) {
        contact.setUpdatedAt(Instant.now());

        int rows = jdbc.update("""
            UPDATE contacts
            SET first_name = ?, last_name = ?, email = ?, phone = ?, company = ?, updated_at = ?
            WHERE id = ?
            """,
            contact.getFirstName(), contact.getLastName(), contact.getEmail(),
            contact.getPhone(), contact.getCompany(),
            Timestamp.from(contact.getUpdatedAt()), contact.getId());
        return rows > 0;
    }

    public boolean delete(String id) {
        int rows = jdbc.update("DELETE FROM contacts WHERE id = ?", id);
        return rows > 0;
    }

    public int count() {
        Integer count = jdbc.queryForObject("SELECT COUNT(*) FROM contacts", Integer.class);
        return count != null ? count : 0;
    }

    public int importContacts(List<Contact> contacts) {
        Instant now = Instant.now();
        for (Contact c : contacts) {
            c.setCreatedAt(now);
            c.setUpdatedAt(now);
            jdbc.update("""
                INSERT INTO contacts (id, first_name, last_name, email, phone, company, created_at, updated_at)
                VALUES (?, ?, ?, ?, ?, ?, ?, ?)
                """,
                c.getId(), c.getFirstName(), c.getLastName(),
                c.getEmail(), c.getPhone(), c.getCompany(),
                Timestamp.from(c.getCreatedAt()), Timestamp.from(c.getUpdatedAt()));
        }
        return contacts.size();
    }

    public List<Contact> exportAll() {
        return jdbc.query("""
            SELECT id, first_name, last_name, email, phone, company, created_at, updated_at
            FROM contacts
            ORDER BY created_at
            """, ROW_MAPPER);
    }
}
