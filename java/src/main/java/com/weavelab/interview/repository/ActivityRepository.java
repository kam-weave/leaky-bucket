package com.weavelab.interview.repository;

import com.weavelab.interview.model.ActivityReport;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.stereotype.Repository;

import java.sql.Timestamp;
import java.time.Instant;
import java.util.ArrayList;
import java.util.HashMap;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;

@Repository
public class ActivityRepository {

    private final JdbcTemplate jdbc;

    public ActivityRepository(JdbcTemplate jdbc) {
        this.jdbc = jdbc;
    }

    public void log(String userId, String action, String resourceType, String resourceId) {
        jdbc.update("""
            INSERT INTO activity_log (user_id, action, resource_type, resource_id, created_at)
            VALUES (?, ?, ?, ?, ?)
            """, userId, action, resourceType, resourceId, Timestamp.from(Instant.now()));
    }

    public List<ActivityReport> generateReport(Instant since) {
        Map<String, ActivityReport> reportMap = new LinkedHashMap<>();

        jdbc.query("""
            SELECT user_id, action, resource_type, COUNT(*) as cnt
            FROM activity_log
            WHERE created_at >= ?
            GROUP BY user_id, action, resource_type
            ORDER BY user_id
            """, rs -> {
            String userId = rs.getString("user_id");
            String action = rs.getString("action");
            String resourceType = rs.getString("resource_type");
            int count = rs.getInt("cnt");

            ActivityReport report = reportMap.computeIfAbsent(userId, id -> {
                ActivityReport r = new ActivityReport();
                r.setUserId(id);
                r.setByAction(new HashMap<>());
                r.setByResource(new HashMap<>());
                return r;
            });

            report.setTotalActions(report.getTotalActions() + count);
            report.getByAction().merge(action, count, Integer::sum);
            report.getByResource().merge(resourceType, count, Integer::sum);
        }, Timestamp.from(since));

        return new ArrayList<>(reportMap.values());
    }
}
